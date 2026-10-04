package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/agent"
	"github.com/Overview-Note/overview/internal/ai"
	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/service"
)

// writeErrorCode writes a JSON error with an explicit machine-readable code.
func writeErrorCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg}})
}

// agentIdentity resolves the caller for agent requests. When authentication is
// disabled the caller is treated as the administrator-equivalent owner.
func (s *Server) agentIdentity(r *http.Request) (userID, username, role string) {
	if u, ok := userFromContext(r.Context()); ok {
		return u.ID, u.Username, u.Role
	}
	return "", "owner", agentPolicyOwner
}

const agentPolicyOwner = "owner"

// agentReady enforces the configuration gates shared by the agent endpoints.
func (s *Server) agentReady(w http.ResponseWriter) bool {
	if s.opts.Agent == nil || s.opts.AI == nil || !s.opts.AI.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "AI is not configured")
		return false
	}
	if !s.opts.AI.AgentEnabled() {
		writeErrorCode(w, http.StatusBadRequest, "agent_disabled", "the AI agent is disabled")
		return false
	}
	if s.opts.AI.ToolCallingMode() == service.ToolCallingOff || s.opts.AI.ToolCallingStatus() == "false" {
		writeErrorCode(w, http.StatusBadRequest, "tool_calling_unsupported", "the configured model does not support tool calling")
		return false
	}
	return true
}

// agentRateLimit throttles agent calls per user (falling back to the client IP
// when there is no authenticated identity).
func (s *Server) agentRateLimit(w http.ResponseWriter, r *http.Request, userID string) bool {
	key := userID
	if key == "" {
		key = clientIP(r)
	}
	if !s.agentLimiter.allow("agent:"+key, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return false
	}
	return true
}

func (s *Server) applyAgentPolicy() {
	s.opts.Agent.SetPolicy(s.opts.AI.ConfirmPolicy(), s.opts.AI.MaxSteps(), s.opts.AI.AllowedTools())
}

type agentRequest struct {
	RunID   string            `json:"runId"`
	Input   string            `json:"input"`
	Context map[string]string `json:"context"`
}

// handleAgent runs one non-streaming agent turn.
func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	userID, username, role := s.agentIdentity(r)
	if !s.agentReady(w) {
		return
	}
	if !s.agentRateLimit(w, r, userID) {
		return
	}
	var req agentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.Input) == "" {
		writeError(w, http.StatusBadRequest, "input is required")
		return
	}
	s.applyAgentPolicy()

	var sess *agent.Session
	if req.RunID != "" {
		existing, ok := s.opts.Agent.Session(req.RunID)
		if !ok {
			writeErrorCode(w, http.StatusNotFound, "run_not_found", "unknown run id")
			return
		}
		sess = existing
	} else {
		sess = s.opts.Agent.Start(userID, username, role)
	}

	res, err := s.opts.Agent.Run(r.Context(), sess, req.Input, req.Context)
	s.writeAgentResult(w, res, err)
}

type agentConfirmRequest struct {
	RunID      string          `json:"runId"`
	ToolCallID string          `json:"toolCallId"`
	Decision   string          `json:"decision"`
	Args       json.RawMessage `json:"args"`
}

// handleAgentConfirm approves or rejects a pending tool call.
func (s *Server) handleAgentConfirm(w http.ResponseWriter, r *http.Request) {
	userID, _, _ := s.agentIdentity(r)
	if !s.agentReady(w) {
		return
	}
	if !s.agentRateLimit(w, r, userID) {
		return
	}
	var req agentConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.ToolCallID) == "" {
		writeError(w, http.StatusBadRequest, "runId and toolCallId are required")
		return
	}
	s.applyAgentPolicy()

	res, err := s.opts.Agent.Confirm(r.Context(), req.RunID, req.ToolCallID, req.Decision, req.Args)
	s.writeAgentResult(w, res, err)
}

type agentStopRequest struct {
	RunID string `json:"runId"`
}

// handleAgentStop discards a session.
func (s *Server) handleAgentStop(w http.ResponseWriter, r *http.Request) {
	userID, _, _ := s.agentIdentity(r)
	if !s.agentRateLimit(w, r, userID) {
		return
	}
	var req agentStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		writeError(w, http.StatusBadRequest, "runId is required")
		return
	}
	if s.opts.Agent != nil {
		s.opts.Agent.Stop(req.RunID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"runId": req.RunID, "status": "stopped"})
}

func (s *Server) writeAgentResult(w http.ResponseWriter, res agent.Result, err error) {
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrToolsUnsupported):
			writeErrorCode(w, http.StatusBadRequest, "tool_calling_unsupported", err.Error())
		case errors.Is(err, core.ErrNotFound):
			writeErrorCode(w, http.StatusNotFound, "run_not_found", err.Error())
		case errors.Is(err, core.ErrInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			s.logger.Warn("agent request failed", "error", err)
			writeError(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, agentResponseBody(res))
}

func agentResponseBody(res agent.Result) map[string]any {
	steps := make([]map[string]any, 0, len(res.Steps))
	for _, st := range res.Steps {
		steps = append(steps, map[string]any{
			"toolCallId": st.ToolCallID,
			"name":       st.Name,
			"args":       st.Args,
			"risk":       st.Risk,
			"status":     st.Status,
			"summary":    st.Summary,
		})
	}
	resp := map[string]any{
		"runId":  res.RunID,
		"status": res.Status,
		"text":   res.Text,
		"steps":  steps,
	}
	if res.Pending != nil {
		resp["pending"] = map[string]any{
			"toolCallId": res.Pending.ToolCallID,
			"name":       res.Pending.Name,
			"args":       res.Pending.Args,
			"preview":    res.Pending.Preview,
			"risk":       res.Pending.Risk,
		}
	}
	return resp
}
