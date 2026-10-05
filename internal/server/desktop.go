package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Overview-Note/overview/internal/update"
)

// DesktopHooks exposes desktop-shell capabilities to the HTTP layer without
// making the server import Wails. The desktop entry point implements it; a
// headless server leaves it nil, which disables the desktop endpoints.
type DesktopHooks interface {
	AutostartEnabled() bool
	SetAutostart(bool) error
	DataDir() string
}

// UpdateChecker reports the latest published release. It matches
// update.CheckLatest and can be replaced in tests.
type UpdateChecker func(ctx context.Context, currentVersion string) (latest, url string, hasUpdate bool, err error)

// desktopSettingsBody renders the desktop shell state for the settings API.
func (s *Server) desktopSettingsBody() map[string]any {
	return map[string]any{
		"autostart": s.opts.DesktopHooks.AutostartEnabled(),
		"dataDir":   s.opts.DesktopHooks.DataDir(),
		"version":   s.opts.Version,
	}
}

// handleDesktopSettingsGet returns the desktop shell settings.
func (s *Server) handleDesktopSettingsGet(w http.ResponseWriter, r *http.Request) {
	if !s.desktopHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, s.desktopSettingsBody())
}

type desktopSettingsRequest struct {
	Autostart *bool `json:"autostart"`
}

// handleDesktopSettingsPut updates the desktop shell settings. Only autostart
// is writable; omitted fields are left unchanged.
func (s *Server) handleDesktopSettingsPut(w http.ResponseWriter, r *http.Request) {
	if !s.desktopHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req desktopSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Autostart != nil {
		if err := s.opts.DesktopHooks.SetAutostart(*req.Autostart); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, s.desktopSettingsBody())
}

// handleDesktopUpdate reports the current and latest published version. It
// never downloads or installs anything.
func (s *Server) handleDesktopUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.opts.LocalOnly {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	check := s.opts.UpdateCheck
	if check == nil {
		check = update.CheckLatest
	}
	latest, url, hasUpdate, err := check(r.Context(), s.opts.Version)
	if err != nil {
		s.logger.Warn("check for updates failed", "error", err)
		writeError(w, http.StatusBadGateway, "update check failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current":   s.opts.Version,
		"latest":    latest,
		"hasUpdate": hasUpdate,
		"url":       url,
	})
}

// desktopHooksAvailable reports whether the desktop settings endpoints can
// serve a request.
func (s *Server) desktopHooksAvailable() bool {
	return s.opts.LocalOnly && s.opts.DesktopHooks != nil
}

// desktopGuard hides the desktop-only endpoints from headless servers by
// answering 404 before authentication or routing, so they are indistinguishable
// from paths that do not exist.
func (s *Server) desktopGuard(next http.Handler) http.Handler {
	if s.opts.LocalOnly {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDesktopPath(r.URL.Path) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isDesktopPath(p string) bool {
	const prefix = "/api/v1/desktop/"
	return len(p) >= len(prefix) && p[:len(prefix)] == prefix
}
