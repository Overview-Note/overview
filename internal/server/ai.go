package server

import (
	"encoding/json"
	"net/http"

	"github.com/Overview-Note/overview/internal/ai"
	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/service"
)

func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	enabled := s.opts.AI != nil && s.opts.AI.Enabled()
	resp := map[string]any{"enabled": enabled}
	if s.opts.AI != nil {
		resp["model"] = s.opts.AI.Model()
		resp["toolCalling"] = s.opts.AI.ToolCallingStatus()
		resp["agentEnabled"] = s.opts.AI.AgentEnabled()
		resp["maxSteps"] = s.opts.AI.MaxSteps()
		resp["confirmPolicy"] = s.opts.AI.ConfirmPolicy()
	} else {
		resp["toolCalling"] = "unknown"
		resp["agentEnabled"] = false
		resp["maxSteps"] = service.DefaultMaxSteps
		resp["confirmPolicy"] = service.DefaultConfirmPolicy
	}
	writeJSON(w, http.StatusOK, resp)
}

// aiSettingsBody renders the effective AI + agent configuration for the API.
func (s *Server) aiSettingsBody() map[string]any {
	if s.opts.AI == nil {
		return map[string]any{
			"baseUrl":       "",
			"model":         "",
			"hasKey":        false,
			"agentEnabled":  false,
			"toolCalling":   service.ToolCallingAuto,
			"confirmPolicy": service.DefaultConfirmPolicy,
			"maxSteps":      service.DefaultMaxSteps,
			"allowedTools":  []string{},
		}
	}
	baseURL, model, hasKey := s.opts.AI.Config()
	agentCfg := s.opts.AI.AgentConfig()
	allowed := agentCfg.AllowedTools
	if allowed == nil {
		allowed = []string{}
	}
	return map[string]any{
		"baseUrl":       baseURL,
		"model":         model,
		"hasKey":        hasKey,
		"agentEnabled":  agentCfg.Enabled,
		"toolCalling":   agentCfg.ToolCalling,
		"confirmPolicy": agentCfg.ConfirmPolicy,
		"maxSteps":      agentCfg.MaxSteps,
		"allowedTools":  allowed,
	}
}

// handleGetAISettings returns the effective AI configuration (admin only).
func (s *Server) handleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.aiSettingsBody())
}

type aiSettingsRequest struct {
	BaseURL       *string   `json:"baseUrl"`
	APIKey        *string   `json:"apiKey"`
	Model         *string   `json:"model"`
	AgentEnabled  *bool     `json:"agentEnabled"`
	ToolCalling   *string   `json:"toolCalling"`
	ConfirmPolicy *string   `json:"confirmPolicy"`
	MaxSteps      *int      `json:"maxSteps"`
	AllowedTools  *[]string `json:"allowedTools"`
}

// handleSaveAISettings persists AI configuration (admin only). All fields are
// optional; omitted fields are left unchanged.
func (s *Server) handleSaveAISettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.AI == nil {
		writeError(w, http.StatusServiceUnavailable, "AI is not available")
		return
	}
	var req aiSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	patch := service.AgentSettingsPatch{
		Enabled:       req.AgentEnabled,
		ToolCalling:   req.ToolCalling,
		ConfirmPolicy: req.ConfirmPolicy,
		MaxSteps:      req.MaxSteps,
		AllowedTools:  req.AllowedTools,
	}
	if err := s.opts.AI.SaveAgentSettings(r.Context(), patch); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if req.BaseURL != nil || req.Model != nil || req.APIKey != nil {
		curBase, curModel, _ := s.opts.AI.Config()
		cfg := service.AIConfig{BaseURL: curBase, Model: curModel}
		if req.BaseURL != nil {
			cfg.BaseURL = *req.BaseURL
		}
		if req.Model != nil {
			cfg.Model = *req.Model
		}
		if req.APIKey != nil {
			cfg.APIKey = *req.APIKey
		}
		if err := s.opts.AI.SaveConfig(r.Context(), cfg); err != nil {
			s.writeDomainError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, s.aiSettingsBody())
}

type aiChatRequest struct {
	Mode     string       `json:"mode"` // chat | organize | complete
	Messages []ai.Message `json:"messages"`
	Content  string       `json:"content"`
}

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if s.opts.AI == nil || !s.opts.AI.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}
	var req aiChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	ctx := r.Context()
	var (
		reply string
		err   error
	)
	switch req.Mode {
	case "organize":
		reply, err = s.opts.AI.Organize(ctx, req.Content)
	case "complete":
		reply, err = s.opts.AI.Complete(ctx, req.Content)
	default:
		reply, err = s.opts.AI.Chat(ctx, req.Messages)
	}
	if err != nil {
		if err == core.ErrInvalid {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.logger.Warn("ai request failed", "error", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reply": reply})
}
