package server

import (
	"encoding/json"
	"net/http"

	"github.com/overview-app/overview/internal/ai"
	"github.com/overview-app/overview/internal/core"
)

func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	enabled := s.opts.AI != nil && s.opts.AI.Enabled()
	resp := map[string]any{"enabled": enabled}
	if enabled {
		resp["model"] = s.opts.AI.Model()
	}
	writeJSON(w, http.StatusOK, resp)
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
