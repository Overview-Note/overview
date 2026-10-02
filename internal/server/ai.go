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
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetAISettings returns the effective AI configuration (admin only).
func (s *Server) handleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.AI == nil {
		writeJSON(w, http.StatusOK, map[string]any{"baseUrl": "", "model": "", "hasKey": false})
		return
	}
	baseURL, model, hasKey := s.opts.AI.Config()
	writeJSON(w, http.StatusOK, map[string]any{
		"baseUrl": baseURL,
		"model":   model,
		"hasKey":  hasKey,
	})
}

type aiSettingsRequest struct {
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
}

// handleSaveAISettings persists AI configuration (admin only).
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
	err := s.opts.AI.SaveConfig(r.Context(), service.AIConfig{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Model:   req.Model,
	})
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	baseURL, model, hasKey := s.opts.AI.Config()
	writeJSON(w, http.StatusOK, map[string]any{
		"baseUrl": baseURL,
		"model":   model,
		"hasKey":  hasKey,
	})
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
