package server

import (
	"encoding/json"
	"net/http"

	"github.com/Overview-Note/overview/internal/service"
)

func emptyMailSettings() map[string]any {
	return map[string]any{
		"host":        "",
		"port":        service.DefaultMailPort,
		"username":    "",
		"from":        "",
		"starttls":    false,
		"hasPassword": false,
		"enabled":     false,
	}
}

func mailSettingsPayload(svc *service.MailService) map[string]any {
	cfg := svc.Config()
	return map[string]any{
		"host":        cfg.Host,
		"port":        cfg.Port,
		"username":    cfg.Username,
		"from":        cfg.From,
		"starttls":    cfg.StartTLS,
		"hasPassword": svc.HasPassword(),
		"enabled":     svc.Enabled(),
	}
}

// handleGetMailSettings returns the effective SMTP configuration (admin only).
// The password is never returned.
func (s *Server) handleGetMailSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Mail == nil {
		writeJSON(w, http.StatusOK, emptyMailSettings())
		return
	}
	writeJSON(w, http.StatusOK, mailSettingsPayload(s.opts.Mail))
}

type mailSettingsRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	StartTLS bool   `json:"starttls"`
}

// handleSaveMailSettings persists SMTP configuration (admin only).
func (s *Server) handleSaveMailSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Mail == nil {
		writeError(w, http.StatusServiceUnavailable, "mail is not available")
		return
	}
	var req mailSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	err := s.opts.Mail.SaveConfig(r.Context(), service.MailConfig{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		From:     req.From,
		StartTLS: req.StartTLS,
	})
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mailSettingsPayload(s.opts.Mail))
}
