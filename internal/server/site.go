package server

import (
	"encoding/json"
	"net/http"

	"github.com/Overview-Note/overview/internal/service"
)

func emptySiteSettings() map[string]any {
	return map[string]any{
		"registrationEnabled": false,
		"loginHint":           "",
		"loginIcp":            "",
		"loginLinkUrl":        "",
		"loginLinkText":       "",
	}
}

func siteSettingsPayload(cfg service.SiteConfig) map[string]any {
	return map[string]any{
		"registrationEnabled": cfg.RegistrationEnabled,
		"loginHint":           cfg.LoginHint,
		"loginIcp":            cfg.LoginICP,
		"loginLinkUrl":        cfg.LoginLinkURL,
		"loginLinkText":       cfg.LoginLinkText,
	}
}

// handleGetSiteSettings returns the public site configuration (admin only).
func (s *Server) handleGetSiteSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Site == nil {
		writeJSON(w, http.StatusOK, emptySiteSettings())
		return
	}
	writeJSON(w, http.StatusOK, siteSettingsPayload(s.opts.Site.Config()))
}

type siteSettingsRequest struct {
	RegistrationEnabled bool   `json:"registrationEnabled"`
	LoginHint           string `json:"loginHint"`
	LoginICP            string `json:"loginIcp"`
	LoginLinkURL        string `json:"loginLinkUrl"`
	LoginLinkText       string `json:"loginLinkText"`
}

// handleSaveSiteSettings persists the public site configuration (admin only).
func (s *Server) handleSaveSiteSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Site == nil {
		writeError(w, http.StatusServiceUnavailable, "site settings are not available")
		return
	}
	var req siteSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	err := s.opts.Site.SaveConfig(r.Context(), service.SiteConfig{
		RegistrationEnabled: req.RegistrationEnabled,
		LoginHint:           req.LoginHint,
		LoginICP:            req.LoginICP,
		LoginLinkURL:        req.LoginLinkURL,
		LoginLinkText:       req.LoginLinkText,
	})
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, siteSettingsPayload(s.opts.Site.Config()))
}
