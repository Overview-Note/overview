package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/service"
)

// maxChangeLimit caps how many note changes a single /sync/changes call returns.
const maxChangeLimit = 5000

// sseHeartbeat is how often a comment line keeps an idle SSE connection alive.
const sseHeartbeat = 25 * time.Second

// handleSyncChanges serves the incremental change feed. Query parameters:
//
//	since    exclusive note sequence cursor (default 0; 0 means full sync)
//	sinceTs  exclusive tombstone timestamp (RFC3339; default zero = all)
//	limit    maximum note changes to return (default 500, max 5000)
//
// Paginate by passing the last returned change's seq back as since. Advance the
// tombstone cursor by passing the response's latestTs back as sinceTs. When
// hasMore is true, call again with the updated since until it is false.
func (s *Server) handleSyncChanges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	since := int64(0)
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "since must be a non-negative integer")
			return
		}
		since = n
	}

	var sinceTs time.Time
	if raw := strings.TrimSpace(q.Get("sinceTs")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "sinceTs must be an RFC3339 timestamp")
			return
		}
		sinceTs = t
	}

	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if n > maxChangeLimit {
			n = maxChangeLimit
		}
		limit = n
	}

	changes, err := s.svc.Changes(r.Context(), since, sinceTs, limit)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, changes)
}

// handleSyncEvents streams change notifications as Server-Sent Events. Each
// payload carries the current change cursors; a client reacts by pulling
// /sync/changes. A comment heartbeat keeps idle connections open.
func (s *Server) handleSyncEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	// Clear the server-wide WriteTimeout for this long-lived stream; otherwise
	// the connection is torn down after ~120s. If the transport cannot honour
	// it, the heartbeat plus client reconnect still recover the stream.
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		s.logger.Debug("sse: cannot clear write deadline", "error", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}

	bus := s.svc.ChangeBus()
	id, events := bus.Subscribe()
	defer bus.Unsubscribe(id)

	if err := writeChangeEvent(w, rc, s.svc.CurrentChangeCursor(r.Context())); err != nil {
		return
	}

	ping := time.NewTicker(sseHeartbeat)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := writeChangeEvent(w, rc, ev); err != nil {
				return
			}
		case <-ping.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}

// writeChangeEvent emits one SSE change event with the current cursors.
func writeChangeEvent(w io.Writer, rc *http.ResponseController, ev service.ChangeEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: change\ndata: %s\n\n", data); err != nil {
		return err
	}
	return rc.Flush()
}

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
