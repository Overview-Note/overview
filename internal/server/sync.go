package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

// SyncHooks exposes desktop vault-sync state and control to the HTTP layer
// without making the server import the sync engine. The desktop shell
// implements it; a headless server leaves it nil, which hides the endpoints.
type SyncHooks interface {
	SyncStatus(ctx context.Context) (SyncStatus, error)
	SyncConfigure(ctx context.Context, cfg SyncConfig) (SyncStatus, error)
	SyncRun(ctx context.Context) error
	SyncConflicts(ctx context.Context) ([]SyncConflict, error)
	SyncResolveConflict(ctx context.Context, id, keep string) error
}

// SyncProgress reports how many sync actions have been applied.
type SyncProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// SyncConflict describes a note whose local and remote copies diverged.
type SyncConflict struct {
	ID            string    `json:"id"`
	OriginalPath  string    `json:"originalPath"`
	ConflictPath  string    `json:"conflictPath"`
	LocalVersion  string    `json:"localVersion"`
	RemoteVersion string    `json:"remoteVersion"`
	DetectedAt    time.Time `json:"detectedAt"`
}

// SyncStatus is the state the desktop UI renders.
type SyncStatus struct {
	Enabled     bool           `json:"enabled"`
	ServerURL   string         `json:"serverURL"`
	VaultID     string         `json:"vaultId"`
	Direction   string         `json:"direction"`
	IntervalSec int            `json:"intervalSec"`
	LastSyncAt  time.Time      `json:"lastSyncAt"`
	Status      string         `json:"status"`
	Progress    SyncProgress   `json:"progress"`
	Conflicts   []SyncConflict `json:"conflicts"`
	LastError   string         `json:"lastError"`
	Connected   bool           `json:"connected"`
}

// SyncConfig carries an update to the sync configuration. Every field is
// optional; omitted fields are left unchanged. The token is write-only and is
// never echoed back.
type SyncConfig struct {
	ServerURL   *string `json:"serverURL"`
	Token       *string `json:"token"`
	Enabled     *bool   `json:"enabled"`
	Direction   *string `json:"direction"`
	IntervalSec *int    `json:"intervalSec"`
}

// syncHooksAvailable reports whether the sync endpoints may serve a request.
func (s *Server) syncHooksAvailable() bool {
	return s.opts.LocalOnly && s.opts.SyncHooks != nil
}

func (s *Server) handleSyncGet(w http.ResponseWriter, r *http.Request) {
	if !s.syncHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	status, err := s.opts.SyncHooks.SyncStatus(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSyncPut(w http.ResponseWriter, r *http.Request) {
	if !s.syncHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var cfg SyncConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if cfg.Direction != nil {
		switch *cfg.Direction {
		case "both", "pull", "push":
		default:
			writeError(w, http.StatusBadRequest, "direction must be one of both, pull, push")
			return
		}
	}
	if cfg.IntervalSec != nil && *cfg.IntervalSec < 0 {
		writeError(w, http.StatusBadRequest, "intervalSec must not be negative")
		return
	}
	status, err := s.opts.SyncHooks.SyncConfigure(r.Context(), cfg)
	if err != nil {
		if errors.Is(err, core.ErrInvalid) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, "sync configuration could not be verified")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSyncRun(w http.ResponseWriter, r *http.Request) {
	if !s.syncHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.opts.SyncHooks.SyncRun(r.Context()); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"started": true})
}

func (s *Server) handleSyncConflicts(w http.ResponseWriter, r *http.Request) {
	if !s.syncHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	conflicts, err := s.opts.SyncHooks.SyncConflicts(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	if conflicts == nil {
		conflicts = []SyncConflict{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}

type resolveConflictRequest struct {
	ID   string `json:"id"`
	Keep string `json:"keep"`
}

func (s *Server) handleSyncResolve(w http.ResponseWriter, r *http.Request) {
	if !s.syncHooksAvailable() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req resolveConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Keep != "local" && req.Keep != "remote" {
		writeError(w, http.StatusBadRequest, "keep must be one of local, remote")
		return
	}
	if err := s.opts.SyncHooks.SyncResolveConflict(r.Context(), req.ID, req.Keep); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"resolved": true})
}
