package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Overview-Note/overview/internal/service"
)

func emptyStorageSettings() map[string]any {
	return map[string]any{
		"endpoint":  "",
		"region":    service.DefaultS3Region,
		"accessKey": "",
		"bucket":    "",
		"useSSL":    true,
		"publicURL": "",
		"hasSecret": false,
		"enabled":   false,
	}
}

func storageSettingsPayload(svc *service.StorageService) map[string]any {
	cfg, hasSecret := svc.Config()
	return map[string]any{
		"endpoint":  cfg.Endpoint,
		"region":    cfg.Region,
		"accessKey": cfg.AccessKey,
		"bucket":    cfg.Bucket,
		"useSSL":    cfg.UseSSL,
		"publicURL": cfg.PublicURL,
		"hasSecret": hasSecret,
		"enabled":   svc.Enabled(),
	}
}

// handleGetStorageSettings returns the effective asset backend configuration
// (admin only). The secret key is never returned.
func (s *Server) handleGetStorageSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Storage == nil {
		writeJSON(w, http.StatusOK, emptyStorageSettings())
		return
	}
	writeJSON(w, http.StatusOK, storageSettingsPayload(s.opts.Storage))
}

type storageSettingsRequest struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Bucket    string `json:"bucket"`
	UseSSL    bool   `json:"useSSL"`
	PublicURL string `json:"publicURL"`
}

func (r storageSettingsRequest) config() service.StorageConfig {
	return service.StorageConfig{
		Endpoint:  r.Endpoint,
		Region:    r.Region,
		AccessKey: r.AccessKey,
		SecretKey: r.SecretKey,
		Bucket:    r.Bucket,
		UseSSL:    r.UseSSL,
		PublicURL: r.PublicURL,
	}
}

// handleSaveStorageSettings persists the asset backend configuration and swaps
// the active store (admin only).
func (s *Server) handleSaveStorageSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "storage settings are not available")
		return
	}
	var req storageSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	err := s.opts.Storage.SaveConfig(r.Context(), req.config())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, storageSettingsPayload(s.opts.Storage))
}

// handleTestStorage probes a configuration without changing the active
// backend (admin only). It accepts an optional body with the same shape as the
// PUT endpoint; when absent the saved configuration is probed. A body with an
// empty secret key reuses the stored secret.
func (s *Server) handleTestStorage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "storage settings are not available")
		return
	}
	var req storageSettingsRequest
	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	var testErr error
	if errors.Is(decodeErr, io.EOF) {
		testErr = s.opts.Storage.Test(r.Context())
	} else {
		testErr = s.opts.Storage.TestConfig(r.Context(), req.config())
	}
	if testErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": testErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": ""})
}
