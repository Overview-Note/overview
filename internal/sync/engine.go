package sync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/markdown"
	"github.com/Overview-Note/overview/internal/server"
)

// Direction controls which side may originate a change.
type Direction string

const (
	DirectionBoth Direction = "both"
	DirectionPull Direction = "pull"
	DirectionPush Direction = "push"
)

// Engine status values reported to the desktop UI.
const (
	StatusIdle    = "idle"
	StatusSyncing = "syncing"
	StatusError   = "error"
	StatusPaused  = "paused"
)

// assetRefPattern mirrors the service-layer pattern so the sync engine extracts
// the same vault-relative attachment paths from note bodies.
var assetRefPattern = regexp.MustCompile(`(?:/api/v1/assets/|/api/assets/|/assets/|assets/)([A-Za-z0-9._/-]+)`)

// Options configures an Engine. Repo, Raw and Assets are required.
type Options struct {
	DataDir string
	Repo    core.NoteRepository
	Raw     core.RawWriter
	Assets  core.AssetStore
	Logger  *slog.Logger
	// UserAgent identifies the client to the server.
	UserAgent string
	// Client injects a pre-built remote client for tests.
	Client *RemoteClient
}

// Engine mirrors a local vault against a remote Overview server.
type Engine struct {
	dataDir  string
	repo     core.NoteRepository
	raw      core.RawWriter
	assets   core.AssetStore
	logger   *slog.Logger
	state    *State
	deviceID string
	ua       string

	cfgMu  sync.Mutex
	client *RemoteClient
	token  string

	statusMu   sync.Mutex
	status     string
	progress   server.SyncProgress
	lastError  string
	warning    string
	connected  bool
	lastSyncAt time.Time

	runMu     sync.Mutex
	runCancel context.CancelFunc
	runDone   chan struct{}

	attemptMu    sync.Mutex
	lastAttempt  time.Time
	forcePending int

	// execMu serializes SyncOnce so the poll loop and the SSE trigger never
	// reconcile concurrently.
	execMu sync.Mutex

	sseMu     sync.Mutex
	sseCancel context.CancelFunc
	sseWake   chan struct{}

	debounceMu    sync.Mutex
	debounceTimer *time.Timer

	wake    chan struct{}
	writeMu sync.Mutex
	recent  map[string]time.Time
}

var _ server.SyncHooks = (*Engine)(nil)

// NewEngine loads the persisted state and token and prepares the engine. It
// does not start any background work; call Start or Run.
func NewEngine(opts Options) (*Engine, error) {
	if opts.Repo == nil || opts.Raw == nil || opts.Assets == nil {
		return nil, core.Invalidf("sync engine requires a repository and asset store")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	state, err := OpenState(opts.DataDir)
	if err != nil {
		return nil, err
	}
	token, err := ReadToken(opts.DataDir)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		dataDir:  opts.DataDir,
		repo:     opts.Repo,
		raw:      opts.Raw,
		assets:   opts.Assets,
		logger:   opts.Logger,
		state:    state,
		deviceID: state.Snapshot().DeviceID,
		ua:       opts.UserAgent,
		token:    token,
		status:   StatusIdle,
		wake:     make(chan struct{}, 1),
		sseWake:  make(chan struct{}, 1),
		recent:   map[string]time.Time{},
	}
	if e.ua == "" {
		e.ua = "Overview-Sync/1.0"
	}
	if opts.Client != nil {
		e.client = opts.Client
	} else {
		e.rebuildClient(state.Snapshot().ServerURL, token)
	}
	return e, nil
}

// rebuildClient constructs the remote client for the given URL, tolerating an
// empty or invalid URL by leaving the client nil.
func (e *Engine) rebuildClient(serverURL, token string) {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		e.client = nil
		return
	}
	client, err := NewRemoteClient(serverURL, ClientOptions{Token: token, UserAgent: e.ua})
	if err != nil {
		e.logger.Warn("sync server URL rejected", "error", err)
		e.client = nil
		return
	}
	e.cfgMu.Lock()
	e.client = client
	e.cfgMu.Unlock()
}

func (e *Engine) currentClient() *RemoteClient {
	e.cfgMu.Lock()
	defer e.cfgMu.Unlock()
	return e.client
}

// ---------------------------------------------------------------------------
// server.SyncHooks
// ---------------------------------------------------------------------------

// SyncStatus returns the current engine state for the desktop UI.
func (e *Engine) SyncStatus(context.Context) (server.SyncStatus, error) {
	st := e.state.Snapshot()
	e.statusMu.Lock()
	status := e.status
	progress := e.progress
	lastErr := e.lastError
	connected := e.connected
	lastSync := e.lastSyncAt
	e.statusMu.Unlock()
	if status == "" {
		status = StatusIdle
	}
	if lastSync.IsZero() {
		lastSync = st.LastSyncAt
	}
	return server.SyncStatus{
		Enabled:     st.Enabled,
		ServerURL:   st.ServerURL,
		VaultID:     st.VaultID,
		Direction:   st.Direction,
		IntervalSec: st.IntervalSec,
		LastSyncAt:  lastSync,
		Status:      status,
		Progress:    progress,
		Conflicts:   toServerConflicts(st.Conflicts),
		LastError:   lastErr,
		Connected:   connected,
	}, nil
}

// SyncConfigure persists a configuration update and verifies connectivity.
func (e *Engine) SyncConfigure(ctx context.Context, cfg server.SyncConfig) (server.SyncStatus, error) {
	st := e.state.Snapshot()
	newURL := st.ServerURL
	if cfg.ServerURL != nil {
		newURL = strings.TrimSpace(*cfg.ServerURL)
	}
	newEnabled := st.Enabled
	if cfg.Enabled != nil {
		newEnabled = *cfg.Enabled
	}
	newDirection := st.Direction
	if cfg.Direction != nil {
		newDirection = *cfg.Direction
	}
	newInterval := st.IntervalSec
	if cfg.IntervalSec != nil {
		newInterval = *cfg.IntervalSec
	}

	if cfg.Token != nil {
		if err := WriteToken(e.dataDir, strings.TrimSpace(*cfg.Token)); err != nil {
			return server.SyncStatus{}, err
		}
	}
	token, err := ReadToken(e.dataDir)
	if err != nil {
		return server.SyncStatus{}, err
	}

	if newURL != "" {
		if _, err := normalizeServerURL(newURL); err != nil {
			return server.SyncStatus{}, err
		}
	}
	if err := e.state.Mutate(func(d *stateData) {
		d.ServerURL = newURL
		d.Enabled = newEnabled
		d.Direction = newDirection
		d.IntervalSec = newInterval
	}); err != nil {
		return server.SyncStatus{}, err
	}
	e.rebuildClient(newURL, token)
	e.restartSSE()

	if newEnabled && newURL != "" {
		client := e.currentClient()
		if client == nil {
			status, _ := e.SyncStatus(ctx)
			return status, core.Invalidf("sync server URL is invalid")
		}
		vaultID, err := client.Validate(ctx)
		if err != nil {
			e.setError(err)
			status, _ := e.SyncStatus(ctx)
			return status, err
		}
		e.setConnected(true)
		e.setError(nil)
		if vaultID != "" {
			_ = e.state.Mutate(func(d *stateData) {
				if d.VaultID == "" {
					d.VaultID = vaultID
				}
			})
		}
	}
	return e.SyncStatus(ctx)
}

// SyncRun starts a sync in the background and returns immediately. Unlike the
// poll timer, an explicit run bypasses and resets the attempt throttle.
func (e *Engine) SyncRun(context.Context) error {
	e.startAsync(true)
	return nil
}

// SyncConflicts lists the unresolved conflicts.
func (e *Engine) SyncConflicts(context.Context) ([]server.SyncConflict, error) {
	return toServerConflicts(e.state.Snapshot().Conflicts), nil
}

// SyncResolveConflict resolves a conflict in favour of the local or remote copy.
func (e *Engine) SyncResolveConflict(ctx context.Context, id, keep string) error {
	st := e.state.Snapshot()
	conflict, ok := findConflict(st.Conflicts, id)
	if !ok {
		return core.ErrNotFound
	}
	client := e.currentClient()
	canPush := Direction(st.Direction) != DirectionPull

	switch keep {
	case "remote":
		// The remote original already owns OriginalPath; drop the copy.
		e.removeLocal(ctx, conflict.ConflictPath)
		if client != nil && canPush {
			if err := e.retry(ctx, func() error { return client.DeleteNote(ctx, conflict.ConflictPath) }); err != nil {
				e.logger.Warn("delete remote conflict copy", "path", conflict.ConflictPath, "error", err)
			}
		}
	case "local":
		localRaw, err := e.repo.Raw(ctx, conflict.ConflictPath)
		if err != nil {
			return err
		}
		doc := markdown.Parse(string(localRaw))
		doc.Meta.ID = conflict.ID
		restored := []byte(doc.String())
		if err := e.raw.WriteRaw(ctx, conflict.OriginalPath, restored); err != nil {
			return err
		}
		e.markWritten(conflict.OriginalPath)
		if client != nil && canPush {
			res, err := client.PutNoteRaw(ctx, conflict.OriginalPath, restored, conflict.RemoteVersion)
			if err != nil {
				return err
			}
			if err := e.state.Mutate(func(d *stateData) {
				d.Notes[conflict.ID] = noteState{Path: conflict.OriginalPath, BaseVersion: res.Version, SyncedAt: time.Now().UTC()}
			}); err != nil {
				return err
			}
		}
		e.removeLocal(ctx, conflict.ConflictPath)
		if client != nil && canPush {
			if err := e.retry(ctx, func() error { return client.DeleteNote(ctx, conflict.ConflictPath) }); err != nil {
				e.logger.Warn("delete remote conflict copy", "path", conflict.ConflictPath, "error", err)
			}
		}
	}
	return e.removeConflict(conflict.ConflictPath)
}

// removeLocal deletes a local note, ignoring a missing file.
func (e *Engine) removeLocal(ctx context.Context, rel string) {
	if err := e.repo.Delete(ctx, rel); err != nil && !errors.Is(err, core.ErrNotFound) {
		e.logger.Warn("delete conflict copy", "path", rel, "error", err)
	}
	e.markWritten(rel)
}

// removeConflict drops the conflict record (and its copy's note state) so the
// engine stops tracking the discarded file.
func (e *Engine) removeConflict(conflictPath string) error {
	return e.state.Mutate(func(d *stateData) {
		out := d.Conflicts[:0]
		for _, c := range d.Conflicts {
			if c.ConflictPath == conflictPath {
				continue
			}
			out = append(out, c)
		}
		d.Conflicts = out
		for id, ns := range d.Notes {
			if ns.Path == conflictPath {
				delete(d.Notes, id)
			}
		}
	})
}

func findConflict(conflicts []Conflict, id string) (Conflict, bool) {
	for _, c := range conflicts {
		if c.ID == id {
			return c, true
		}
	}
	return Conflict{}, false
}

func toServerConflicts(in []Conflict) []server.SyncConflict {
	out := make([]server.SyncConflict, 0, len(in))
	for _, c := range in {
		out = append(out, server.SyncConflict{
			ID: c.ID, OriginalPath: c.OriginalPath, ConflictPath: c.ConflictPath,
			LocalVersion: c.LocalVersion, RemoteVersion: c.RemoteVersion, DetectedAt: c.DetectedAt,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// lifecycle
// ---------------------------------------------------------------------------

// Start launches the background loop. It is safe to call more than once.
func (e *Engine) Start(context.Context) {
	e.runMu.Lock()
	if e.runCancel != nil {
		e.runMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.runCancel = cancel
	done := make(chan struct{})
	e.runDone = done
	e.runMu.Unlock()
	go func() {
		defer close(done)
		if err := e.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			e.logger.Warn("sync loop stopped", "error", err)
		}
	}()
}

// Stop cancels the background loop and waits for it.
func (e *Engine) Stop(ctx context.Context) error {
	e.runMu.Lock()
	if e.runCancel != nil {
		e.runCancel()
		e.runCancel = nil
	}
	done := e.runDone
	e.runDone = nil
	e.runMu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
	return nil
}

// Run subscribes to the server's change stream and polls on the configured
// interval as a fallback, syncing whenever the stream signals a change or a
// dirty path is signalled. It blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	e.syncIfEnabled(ctx)
	go e.runSSE(ctx)
	for {
		interval := e.interval()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
			e.syncIfEnabled(ctx)
		case <-e.wake:
			timer.Stop()
			e.syncIfEnabled(ctx)
		}
	}
}

// sseDebounce coalesces a burst of change events into a single reconciliation.
const sseDebounce = 500 * time.Millisecond

// restartSSE cancels the active subscription so the loop reconnects with the
// current client and configuration.
func (e *Engine) restartSSE() {
	e.sseMu.Lock()
	cancel := e.sseCancel
	e.sseMu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case e.sseWake <- struct{}{}:
	default:
	}
}

// runSSE keeps a change-stream subscription alive for as long as sync is
// enabled. The poll loop in Run remains the correctness fallback when the
// stream is unavailable or disconnected.
func (e *Engine) runSSE(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		st := e.state.Snapshot()
		client := e.currentClient()
		if !st.Enabled || client == nil {
			select {
			case <-ctx.Done():
				return
			case <-e.sseWake:
			case <-time.After(time.Second):
			}
			continue
		}

		subCtx, cancel := context.WithCancel(ctx)
		e.sseMu.Lock()
		e.sseCancel = cancel
		e.sseMu.Unlock()
		err := client.Events(subCtx, func(seq int, ts string) {
			e.onRemoteEvent(subCtx, seq, ts)
		})
		cancel()
		e.sseMu.Lock()
		e.sseCancel = nil
		e.sseMu.Unlock()

		if ctx.Err() != nil {
			return
		}
		if err != nil && (errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)) {
			e.logger.Warn("sync change stream unauthorized; waiting for reconfiguration", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-e.sseWake:
			case <-time.After(30 * time.Second):
			}
			continue
		}
		// Events reconnects internally; a short pause avoids a hot loop if it
		// returns immediately.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// onRemoteEvent debounces a stream event into a single triggered sync.
func (e *Engine) onRemoteEvent(ctx context.Context, _ int, _ string) {
	e.debounceMu.Lock()
	if e.debounceTimer != nil {
		e.debounceTimer.Stop()
	}
	e.debounceTimer = time.AfterFunc(sseDebounce, func() {
		e.debounceMu.Lock()
		e.debounceTimer = nil
		e.debounceMu.Unlock()
		e.sseTriggeredSync(ctx)
	})
	e.debounceMu.Unlock()
}

// sseTriggeredSync runs a full reconciliation when the remote cursor has moved
// since the last successful sync. The /sync/changes head is only a fast
// "is anything new" check; correctness still comes from the manifest three-way
// comparison inside SyncOnce.
func (e *Engine) sseTriggeredSync(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	st := e.state.Snapshot()
	if !st.Enabled {
		return
	}
	client := e.currentClient()
	if client == nil {
		return
	}
	changed, err := e.remoteChanged(ctx, client, st)
	if err != nil {
		e.logger.Debug("sync change check failed", "error", err)
	} else if !changed {
		return
	}
	if err := e.SyncOnce(ctx); err != nil && ctx.Err() == nil {
		e.logger.Warn("event-driven sync failed", "error", err)
	}
}

// remoteChanged reports whether the remote change feed has advanced past the
// stored cursor. Any error is treated as "changed" so a failed probe never
// causes a missed sync.
func (e *Engine) remoteChanged(ctx context.Context, client *RemoteClient, st stateData) (bool, error) {
	if st.LastSyncAt.IsZero() {
		return true, nil
	}
	changes, err := client.Changes(ctx, st.Cursor.LatestSeq, cursorTimestamp(st.Cursor.LatestTs), 1)
	if err != nil {
		return true, err
	}
	if changes.LatestSeq != st.Cursor.LatestSeq {
		return true, nil
	}
	if !changes.LatestTs.Equal(st.Cursor.LatestTs) {
		return true, nil
	}
	if len(changes.Tombstones) > 0 || len(changes.FolderTombstones) > 0 {
		return true, nil
	}
	return false, nil
}

// cursorTimestamp renders a cursor timestamp as the RFC3339 string the server
// expects, or "" for the zero time.
func cursorTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// NotifyDirty signals that a local path changed. Self-writes performed by the
// engine are ignored so a pull does not trigger an endless loop.
func (e *Engine) NotifyDirty(rel string) {
	if rel == "" || e.recentlyWritten(rel) {
		return
	}
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *Engine) recentlyWritten(rel string) bool {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	at, ok := e.recent[rel]
	if !ok {
		return false
	}
	if time.Since(at) > 3*time.Second {
		delete(e.recent, rel)
		return false
	}
	return true
}

func (e *Engine) markWritten(rel string) {
	e.writeMu.Lock()
	e.recent[rel] = time.Now()
	e.writeMu.Unlock()
}

func (e *Engine) interval() time.Duration {
	st := e.state.Snapshot()
	sec := st.IntervalSec
	if sec <= 0 {
		sec = DefaultIntervalSec
	}
	return time.Duration(sec) * time.Second
}

func (e *Engine) syncIfEnabled(ctx context.Context) {
	st := e.state.Snapshot()
	if !st.Enabled || e.currentClient() == nil {
		return
	}
	e.attemptMu.Lock()
	if e.forcePending > 0 {
		e.forcePending--
	}
	pending := e.forcePending
	e.lastAttempt = time.Now()
	e.attemptMu.Unlock()

	if pending > 0 {
		select {
		case e.wake <- struct{}{}:
		default:
		}
	}

	if err := e.SyncOnce(ctx); err != nil && ctx.Err() == nil {
		e.logger.Warn("sync failed", "error", err)
	}
}

// startAsync launches a one-off sync unless one is already running. A forced
// request bypasses and resets the one-second attempt throttle so explicit runs
// are never coalesced away; the poll timer keeps the throttle.
func (e *Engine) startAsync(force bool) {
	e.runMu.Lock()
	if e.runCancel != nil {
		// A loop is running; wake it. A forced request sets the flag the loop
		// consumes to bypass the throttle on its next pass.
		e.runMu.Unlock()
		if force {
			e.attemptMu.Lock()
			e.forcePending++
			e.attemptMu.Unlock()
		}
		select {
		case e.wake <- struct{}{}:
		default:
		}
		return
	}
	e.runMu.Unlock()

	e.attemptMu.Lock()
	if force {
		e.forcePending++
	}
	forced := e.forcePending > 0
	if forced {
		e.forcePending--
	}
	if !forced && time.Since(e.lastAttempt) < time.Second {
		e.attemptMu.Unlock()
		return
	}
	e.lastAttempt = time.Now()
	e.attemptMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := e.SyncOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			e.logger.Warn("manual sync failed", "error", err)
		}
	}()
}

// ---------------------------------------------------------------------------
// sync loop
// ---------------------------------------------------------------------------

type localNote struct {
	id      string
	path    string
	version string
	body    string
}

// SyncOnce runs a single three-way reconciliation. It is single-flight: the
// poll loop and the SSE trigger serialize on execMu.
func (e *Engine) SyncOnce(ctx context.Context) error {
	e.execMu.Lock()
	defer e.execMu.Unlock()

	st := e.state.Snapshot()
	if !st.Enabled {
		e.setStatus(StatusIdle)
		return nil
	}
	client := e.currentClient()
	if client == nil {
		return nil
	}

	e.setStatus(StatusSyncing)
	e.setProgress(0, 0)
	e.setError(nil)
	e.clearWarning()

	local, err := e.scanLocal(ctx)
	if err != nil {
		e.fail(err)
		return err
	}

	manifest, err := e.fetchManifest(ctx, client)
	if err != nil {
		e.fail(err)
		return err
	}
	if manifest.VaultID != "" {
		if st.VaultID == "" {
			_ = e.state.Mutate(func(d *stateData) {
				if d.VaultID == "" {
					d.VaultID = manifest.VaultID
				}
			})
		} else if manifest.VaultID != st.VaultID {
			err := fmt.Errorf("remote vault %s does not match the configured vault %s", manifest.VaultID, st.VaultID)
			e.fail(err)
			return err
		}
	}

	head := e.fetchChanges(ctx, client, st)
	actions := e.plan(ctx, st, local, manifest, folderTombstones(head))
	e.setProgress(0, len(actions))
	for i, act := range actions {
		if err := ctx.Err(); err != nil {
			e.fail(err)
			return err
		}
		if err := e.execute(ctx, client, act); err != nil {
			if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
				e.setStatus(StatusPaused)
				e.setError(err)
				return err
			}
			e.fail(err)
			return err
		}
		e.setProgress(i+1, len(actions))
	}

	now := time.Now().UTC()
	_ = e.state.Mutate(func(d *stateData) {
		d.LastSyncAt = now
		if head != nil {
			// Only advance to the head observed before the actions ran. A
			// change that arrived during the sync is deliberately left ahead of
			// the cursor so its event retriggers a reconciliation.
			d.Cursor = cursorState{LatestSeq: head.LatestSeq, LatestTs: head.LatestTs}
		}
	})
	e.statusMu.Lock()
	e.lastSyncAt = now
	e.connected = true
	e.status = StatusIdle
	if e.warning != "" {
		e.lastError = e.warning
	} else {
		e.lastError = ""
	}
	e.warning = ""
	e.statusMu.Unlock()
	return nil
}

// fetchChanges reads the remote change head, using the stored cursor so the
// response is bounded. It is best effort: an older server without the endpoint
// still syncs via the manifest.
func (e *Engine) fetchChanges(ctx context.Context, client *RemoteClient, st stateData) *core.SyncChanges {
	changes, err := client.Changes(ctx, st.Cursor.LatestSeq, cursorTimestamp(st.Cursor.LatestTs), 1)
	if err != nil {
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrNotSupported) {
			e.logger.Debug("sync change query failed", "error", err)
		}
		return nil
	}
	return &changes
}

// folderTombstones indexes the folder deletion markers from a change response
// by path and deletion time.
func folderTombstones(changes *core.SyncChanges) map[string]time.Time {
	if changes == nil || len(changes.FolderTombstones) == 0 {
		return nil
	}
	out := make(map[string]time.Time, len(changes.FolderTombstones))
	for _, t := range changes.FolderTombstones {
		out[t.Path] = t.DeletedAt
	}
	return out
}

// fetchManifest fetches the full inventory. The manifest ETag does not cover
// folder membership, so a conditional request could return a 304 with stale
// folders; the client therefore always fetches fresh. Bandwidth is saved by the
// /sync/changes check on the event-driven path instead.
func (e *Engine) fetchManifest(ctx context.Context, client *RemoteClient) (*core.VaultManifest, error) {
	manifest, _, err := client.Manifest(ctx, "")
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

// scanLocal walks the vault, assigning a ULID to every note missing a
// frontmatter id and returning the notes keyed by id.
func (e *Engine) scanLocal(ctx context.Context) (map[string]localNote, error) {
	var notes []core.Note
	if err := e.repo.Walk(ctx, func(n core.Note) error {
		notes = append(notes, n)
		return nil
	}); err != nil {
		return nil, err
	}
	out := make(map[string]localNote, len(notes))
	for _, n := range notes {
		if n.ID == "" {
			id, version, body, err := e.assignID(ctx, n.Path)
			if err != nil {
				return nil, err
			}
			n.ID, n.Version, n.Body = id, version, body
		}
		if _, dup := out[n.ID]; dup {
			e.logger.Warn("duplicate note id in vault", "id", n.ID, "path", n.Path)
			continue
		}
		out[n.ID] = localNote{id: n.ID, path: n.Path, version: n.Version, body: n.Body}
	}
	return out, nil
}

// assignID generates an id for a note without frontmatter and writes it back.
func (e *Engine) assignID(ctx context.Context, rel string) (string, string, string, error) {
	raw, err := e.repo.Raw(ctx, rel)
	if err != nil {
		return "", "", "", err
	}
	doc := markdown.Parse(string(raw))
	doc.Meta.ID = ulid.Make().String()
	updated := []byte(doc.String())
	if err := e.raw.WriteRaw(ctx, rel, updated); err != nil {
		return "", "", "", err
	}
	e.markWritten(rel)
	return doc.Meta.ID, hashBytes(updated), doc.Body, nil
}

// ---------------------------------------------------------------------------
// planning
// ---------------------------------------------------------------------------

type actKind int

const (
	actMkdirLocal actKind = iota
	actMkdirRemote
	actFolderSync
	actFolderDeleteLocal
	actFolderDeleteRemote
	actFolderForget
	actPull
	actPush
	actPullMove
	actPushMove
	actPullDelete
	actPushDelete
	actConflict
)

type action struct {
	kind          actKind
	id            string
	path          string
	from          string
	to            string
	write         bool
	empty         bool
	base          string
	remoteVersion string
	localVersion  string
	body          string
}

// plan computes the ordered action list: folder creates (parent first), pulls,
// pushes, note deletes, then folder deletes (child first) so a folder is only
// removed once its contents have been reconciled.
func (e *Engine) plan(ctx context.Context, st stateData, local map[string]localNote, manifest *core.VaultManifest, tombstones map[string]time.Time) []action {
	canPull := Direction(st.Direction) != DirectionPush
	canPush := Direction(st.Direction) != DirectionPull

	entries, err := e.repo.List(ctx)
	if err != nil {
		e.logger.Warn("list local folders", "error", err)
	}
	localDirs := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir {
			localDirs[entry.Path] = true
		}
	}
	remoteDirs := map[string]bool{}
	for _, f := range manifest.Folders {
		remoteDirs[f] = true
	}
	baseDirs := make(map[string]bool, len(st.Folders))
	for p := range st.Folders {
		baseDirs[p] = true
	}
	remoteNotePaths := make(map[string]bool, len(manifest.Notes))
	for _, n := range manifest.Notes {
		remoteNotePaths[n.Path] = true
	}

	var creates, folderDeletes, folderSync, forgets []action
	seen := map[string]bool{}
	consider := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		l, r, b := localDirs[p], remoteDirs[p], baseDirs[p]
		if at, ok := tombstones[p]; ok && r && at.After(manifest.GeneratedAt) {
			// The folder was deleted after this manifest was generated, so the
			// tombstone is newer and wins over the stale listing.
			r = false
		}
		switch {
		case l && r:
			folderSync = append(folderSync, action{kind: actFolderSync, path: p})
		case l && !r:
			switch {
			case b:
				if canPull {
					folderDeletes = append(folderDeletes, action{kind: actFolderDeleteLocal, path: p})
				}
			case canPush:
				creates = append(creates, action{kind: actMkdirRemote, path: p})
			}
		case !l && r:
			switch {
			case b:
				if canPush && remoteFolderEmpty(p, remoteDirs, remoteNotePaths) {
					folderDeletes = append(folderDeletes, action{kind: actFolderDeleteRemote, path: p, empty: true})
				}
			case canPull:
				creates = append(creates, action{kind: actMkdirLocal, path: p})
			}
		default:
			if b {
				forgets = append(forgets, action{kind: actFolderForget, path: p})
			}
		}
	}
	for p := range localDirs {
		consider(p)
	}
	for p := range remoteDirs {
		consider(p)
	}
	for p := range baseDirs {
		consider(p)
	}
	sortByDepth(creates)
	sortByDepthDesc(folderDeletes)

	remote := make(map[string]core.ManifestNote, len(manifest.Notes))
	for _, n := range manifest.Notes {
		if n.ID == "" {
			continue
		}
		remote[n.ID] = n
	}

	var pulls, pushes, deletes []action
	ids := make(map[string]struct{}, len(local)+len(remote)+len(st.Notes))
	for id := range local {
		ids[id] = struct{}{}
	}
	for id := range remote {
		ids[id] = struct{}{}
	}
	for id := range st.Notes {
		ids[id] = struct{}{}
	}

	for id := range ids {
		l, hasLocal := local[id]
		r, hasRemote := remote[id]
		b, hasBase := st.Notes[id]

		switch {
		case !hasBase:
			switch {
			case hasLocal && hasRemote:
				if l.version == r.Version {
					if l.path != r.Path {
						if canPull {
							pulls = append(pulls, action{kind: actPullMove, id: id, from: l.path, to: r.Path, remoteVersion: r.Version})
						} else if canPush {
							pushes = append(pushes, action{kind: actPushMove, id: id, from: r.Path, to: l.path, base: "*", localVersion: l.version})
						}
					}
				} else if canPull && canPush {
					pulls = append(pulls, action{kind: actConflict, id: id, path: l.path, to: r.Path, remoteVersion: r.Version, localVersion: l.version, body: l.body})
				} else if canPull {
					pulls = append(pulls, action{kind: actPull, id: id, path: r.Path, remoteVersion: r.Version})
				} else if canPush {
					base := "*"
					if l.path == r.Path {
						base = r.Version
					}
					pushes = append(pushes, action{kind: actPush, id: id, path: l.path, base: base, localVersion: l.version, body: l.body})
				}
			case hasLocal:
				if canPush {
					pushes = append(pushes, action{kind: actPush, id: id, path: l.path, base: "*", localVersion: l.version, body: l.body})
				}
			case hasRemote:
				if canPull {
					pulls = append(pulls, action{kind: actPull, id: id, path: r.Path, remoteVersion: r.Version})
				}
			}

		case hasLocal && hasRemote:
			if l.version == r.Version {
				if l.path != r.Path {
					switch {
					case l.path != b.Path && r.Path == b.Path:
						if canPush {
							pushes = append(pushes, action{kind: actPushMove, id: id, from: b.Path, to: l.path, base: b.BaseVersion, localVersion: l.version})
						}
					case r.Path != b.Path && l.path == b.Path:
						if canPull {
							pulls = append(pulls, action{kind: actPullMove, id: id, from: l.path, to: r.Path, remoteVersion: r.Version})
						}
					default:
						if canPull {
							pulls = append(pulls, action{kind: actPullMove, id: id, from: l.path, to: r.Path, remoteVersion: r.Version})
						} else if canPush {
							pushes = append(pushes, action{kind: actPushMove, id: id, from: r.Path, to: l.path, base: b.BaseVersion, localVersion: l.version})
						}
					}
				}
				break
			}
			localChanged := l.version != b.BaseVersion
			remoteChanged := r.Version != b.BaseVersion
			switch {
			case !localChanged && remoteChanged:
				if canPull {
					if r.Path != b.Path && l.path == b.Path {
						pulls = append(pulls, action{kind: actPullMove, id: id, from: l.path, to: r.Path, write: true, remoteVersion: r.Version})
					} else {
						pulls = append(pulls, action{kind: actPull, id: id, path: r.Path, remoteVersion: r.Version})
					}
				}
			case localChanged && !remoteChanged:
				if canPush {
					if l.path != b.Path && r.Path == b.Path {
						pushes = append(pushes, action{kind: actPushMove, id: id, from: b.Path, to: l.path, write: true, base: b.BaseVersion, localVersion: l.version})
					} else {
						pushes = append(pushes, action{kind: actPush, id: id, path: l.path, base: b.BaseVersion, localVersion: l.version, body: l.body})
					}
				}
			default:
				switch {
				case canPull && canPush:
					pulls = append(pulls, action{kind: actConflict, id: id, path: l.path, to: r.Path, remoteVersion: r.Version, localVersion: l.version, body: l.body})
				case canPull:
					pulls = append(pulls, action{kind: actPull, id: id, path: r.Path, remoteVersion: r.Version})
				case canPush:
					pushes = append(pushes, action{kind: actPush, id: id, path: l.path, base: r.Version, localVersion: l.version, body: l.body})
				}
			}

		case hasLocal && !hasRemote:
			// The remote note was deleted.
			if l.version != b.BaseVersion {
				// Local edit survives: resurrect the note on the remote.
				if canPush {
					pushes = append(pushes, action{kind: actPush, id: id, path: l.path, base: "*", localVersion: l.version, body: l.body})
				}
			} else if canPull {
				deletes = append(deletes, action{kind: actPullDelete, id: id, path: b.Path})
			}

		case !hasLocal && hasRemote:
			// The local note was deleted.
			if r.Version != b.BaseVersion {
				if canPull {
					pulls = append(pulls, action{kind: actPull, id: id, path: r.Path, remoteVersion: r.Version})
				}
			} else if canPush {
				deletes = append(deletes, action{kind: actPushDelete, id: id, path: r.Path})
			}
		}
	}

	sortByDepth(pulls)
	sortByDepth(pushes)
	sortByDepth(deletes)

	out := make([]action, 0, len(creates)+len(folderSync)+len(pulls)+len(pushes)+len(deletes)+len(folderDeletes)+len(forgets))
	out = append(out, creates...)
	out = append(out, folderSync...)
	out = append(out, pulls...)
	out = append(out, pushes...)
	out = append(out, deletes...)
	out = append(out, folderDeletes...)
	out = append(out, forgets...)
	return out
}

// remoteFolderEmpty reports whether a remote folder holds no notes or
// subfolders, so a folder deletion never removes remote content.
func remoteFolderEmpty(dir string, dirs, notes map[string]bool) bool {
	prefix := dir + "/"
	for p := range dirs {
		if p != dir && strings.HasPrefix(p, prefix) {
			return false
		}
	}
	for p := range notes {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	return true
}

// sortByDepth orders actions so parent paths (fewer separators) run first.
func sortByDepth(actions []action) {
	for i := 1; i < len(actions); i++ {
		for j := i; j > 0 && pathDepth(actionPath(actions[j])) < pathDepth(actionPath(actions[j-1])); j-- {
			actions[j], actions[j-1] = actions[j-1], actions[j]
		}
	}
}

// sortByDepthDesc orders actions so deeper paths (children) run before their
// parents, used for folder deletions.
func sortByDepthDesc(actions []action) {
	for i := 1; i < len(actions); i++ {
		for j := i; j > 0 && pathDepth(actionPath(actions[j])) > pathDepth(actionPath(actions[j-1])); j-- {
			actions[j], actions[j-1] = actions[j-1], actions[j]
		}
	}
}

func actionPath(a action) string {
	if a.path != "" {
		return a.path
	}
	return a.to
}

func pathDepth(p string) int { return strings.Count(p, "/") }

// ---------------------------------------------------------------------------
// execution
// ---------------------------------------------------------------------------

func (e *Engine) execute(ctx context.Context, client *RemoteClient, act action) error {
	switch act.kind {
	case actMkdirLocal:
		if err := e.repo.Mkdir(ctx, act.path); err != nil {
			return err
		}
		e.markWritten(act.path)
		return e.recordFolderBase(act.path)

	case actMkdirRemote:
		if err := e.retry(ctx, func() error { return client.Mkdir(ctx, act.path) }); err != nil {
			return err
		}
		return e.recordFolderBase(act.path)

	case actFolderSync:
		return e.recordFolderBase(act.path)

	case actFolderDeleteLocal:
		if !e.localFolderEmpty(ctx, act.path) {
			return nil
		}
		if err := e.repo.Delete(ctx, act.path); err != nil && !errors.Is(err, core.ErrNotFound) {
			return err
		}
		e.markWritten(act.path)
		return e.forgetFolder(act.path)

	case actFolderDeleteRemote:
		if !act.empty {
			return nil
		}
		if err := e.retry(ctx, func() error { return client.DeleteNote(ctx, act.path) }); err != nil {
			if errors.Is(err, ErrNotFound) {
				return e.forgetFolder(act.path)
			}
			return err
		}
		return e.forgetFolder(act.path)

	case actFolderForget:
		return e.forgetFolder(act.path)

	case actPull:
		return e.execPull(ctx, client, act.path, act.id, act.remoteVersion)

	case actPush:
		return e.execPush(ctx, client, act.path, act.id, act.base)

	case actPullMove:
		return e.execPullMove(ctx, client, act)

	case actPushMove:
		return e.execPushMove(ctx, client, act)

	case actPullDelete:
		e.removeLocal(ctx, act.path)
		return e.state.Mutate(func(d *stateData) { delete(d.Notes, act.id) })

	case actPushDelete:
		if err := e.retry(ctx, func() error { return client.DeleteNote(ctx, act.path) }); err != nil {
			return err
		}
		return e.state.Mutate(func(d *stateData) { delete(d.Notes, act.id) })

	case actConflict:
		return e.execConflict(ctx, client, act)
	}
	return nil
}

// recordFolderBase marks a folder as synchronized so a later disappearance on
// one side can be told apart from a folder that was never shared. Recording an
// already-known folder is a no-op, avoiding a state rewrite on every sync.
func (e *Engine) recordFolderBase(path string) error {
	if _, ok := e.state.Snapshot().Folders[path]; ok {
		return nil
	}
	return e.state.Mutate(func(d *stateData) {
		d.Folders[path] = folderState{SyncedAt: time.Now().UTC()}
	})
}

// forgetFolder drops a folder from the base after it disappeared on both sides
// or was deleted.
func (e *Engine) forgetFolder(path string) error {
	return e.state.Mutate(func(d *stateData) { delete(d.Folders, path) })
}

// localFolderEmpty reports whether a local folder holds no entries. The folder
// itself appears in the listing and is ignored.
func (e *Engine) localFolderEmpty(ctx context.Context, rel string) bool {
	entries, err := e.repo.List(ctx)
	if err != nil {
		return false
	}
	prefix := rel + "/"
	for _, entry := range entries {
		if entry.Path == rel {
			continue
		}
		if strings.HasPrefix(entry.Path, prefix) {
			return false
		}
	}
	return true
}

func (e *Engine) execPull(ctx context.Context, client *RemoteClient, rel, id, expectVersion string) error {
	raw, version, err := client.GetNoteRaw(ctx, rel)
	if err != nil {
		return err
	}
	if err := e.raw.WriteRaw(ctx, rel, raw); err != nil {
		return err
	}
	e.markWritten(rel)
	if version == "" {
		version = expectVersion
	}
	if err := e.state.Mutate(func(d *stateData) {
		d.Notes[id] = noteState{Path: rel, BaseVersion: version, SyncedAt: time.Now().UTC()}
	}); err != nil {
		return err
	}
	e.pullAssets(ctx, client, string(raw))
	return nil
}

func (e *Engine) execPush(ctx context.Context, client *RemoteClient, rel, id, base string) error {
	raw, err := e.repo.Raw(ctx, rel)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil
		}
		return err
	}
	var res NoteWriteResult
	err = e.retry(ctx, func() error {
		var opErr error
		res, opErr = client.PutNoteRaw(ctx, rel, raw, base)
		return opErr
	})
	if err != nil {
		return err
	}
	if err := e.state.Mutate(func(d *stateData) {
		d.Notes[id] = noteState{Path: rel, BaseVersion: res.Version, SyncedAt: time.Now().UTC()}
	}); err != nil {
		return err
	}
	e.pushAssets(ctx, client, string(raw))
	return nil
}

func (e *Engine) execPullMove(ctx context.Context, client *RemoteClient, act action) error {
	if act.from != act.to {
		if err := e.repo.Move(ctx, act.from, act.to); err != nil {
			if !errors.Is(err, core.ErrNotFound) {
				return err
			}
		}
		e.markWritten(act.from)
		e.markWritten(act.to)
	}
	version := act.remoteVersion
	if act.write {
		raw, v, err := client.GetNoteRaw(ctx, act.to)
		if err != nil {
			return err
		}
		if err := e.raw.WriteRaw(ctx, act.to, raw); err != nil {
			return err
		}
		e.markWritten(act.to)
		if v != "" {
			version = v
		}
		e.pullAssets(ctx, client, string(raw))
	}
	return e.state.Mutate(func(d *stateData) {
		d.Notes[act.id] = noteState{Path: act.to, BaseVersion: version, SyncedAt: time.Now().UTC()}
	})
}

func (e *Engine) execPushMove(ctx context.Context, client *RemoteClient, act action) error {
	if act.from != act.to {
		if err := e.retry(ctx, func() error { return client.Rename(ctx, act.from, act.to) }); err != nil {
			return err
		}
	}
	if !act.write {
		version := act.base
		if version == "" {
			version = act.localVersion
		}
		return e.state.Mutate(func(d *stateData) {
			d.Notes[act.id] = noteState{Path: act.to, BaseVersion: version, SyncedAt: time.Now().UTC()}
		})
	}
	return e.execPush(ctx, client, act.to, act.id, act.base)
}

// execConflict preserves the remote original at the original path and writes
// the local copy to a conflict file that is itself pushed as a new note.
func (e *Engine) execConflict(ctx context.Context, client *RemoteClient, act action) error {
	localRaw, err := e.repo.Raw(ctx, act.path)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return e.execPull(ctx, client, act.to, act.id, act.remoteVersion)
		}
		return err
	}
	remoteRaw, remoteVersion, err := client.GetNoteRaw(ctx, act.to)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return e.execPush(ctx, client, act.path, act.id, "*")
		}
		return err
	}

	conflictID, conflictPath, conflictRaw, err := e.writeConflictCopy(ctx, act.path, localRaw)
	if err != nil {
		return err
	}
	if Direction(e.state.Snapshot().Direction) != DirectionPull {
		res, perr := client.PutNoteRaw(ctx, conflictPath, conflictRaw, "*")
		if perr != nil {
			e.logger.Warn("push conflict copy failed", "path", conflictPath, "error", perr)
		} else {
			_ = e.state.Mutate(func(d *stateData) {
				d.Notes[conflictID] = noteState{Path: conflictPath, BaseVersion: res.Version, SyncedAt: time.Now().UTC()}
			})
		}
	}

	if err := e.raw.WriteRaw(ctx, act.to, remoteRaw); err != nil {
		return err
	}
	e.markWritten(act.path)
	e.markWritten(act.to)
	if err := e.state.Mutate(func(d *stateData) {
		d.Notes[act.id] = noteState{Path: act.to, BaseVersion: remoteVersion, SyncedAt: time.Now().UTC()}
		d.Conflicts = append(d.Conflicts, Conflict{
			ID:            act.id,
			OriginalPath:  act.to,
			ConflictPath:  conflictPath,
			LocalVersion:  act.localVersion,
			RemoteVersion: remoteVersion,
			DetectedAt:    time.Now().UTC(),
		})
	}); err != nil {
		return err
	}
	e.pullAssets(ctx, client, string(remoteRaw))
	return nil
}

// writeConflictCopy writes the local bytes to a sibling conflict file carrying a
// fresh id.
func (e *Engine) writeConflictCopy(ctx context.Context, original string, localRaw []byte) (string, string, []byte, error) {
	doc := markdown.Parse(string(localRaw))
	id := ulid.Make().String()
	doc.Meta.ID = id
	raw := []byte(doc.String())
	conflictPath := conflictPathFor(original, e.deviceID)
	if err := e.raw.WriteRaw(ctx, conflictPath, raw); err != nil {
		return "", "", nil, err
	}
	e.markWritten(conflictPath)
	return id, conflictPath, raw, nil
}

// conflictPathFor builds "<dir>/<name> (conflict-<device>-<ts>).md".
func conflictPathFor(original, device string) string {
	dir := path.Dir(original)
	if dir == "." {
		dir = ""
	}
	base := path.Base(original)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	short := device
	if len(short) > 8 {
		short = short[:8]
	}
	name := fmt.Sprintf("%s (conflict-%s-%s)%s", stem, short, time.Now().UTC().Format("20060102T150405Z"), ext)
	return path.Join(dir, name)
}

// ---------------------------------------------------------------------------
// assets
// ---------------------------------------------------------------------------

// pushAssets uploads the attachments referenced by a note when their local
// content differs from the last synced copy.
func (e *Engine) pushAssets(ctx context.Context, client *RemoteClient, noteContent string) {
	refs := assetRefs(noteContent)
	if len(refs) == 0 {
		return
	}
	st := e.state.Snapshot()
	for _, rel := range refs {
		reader, err := e.assets.Open(ctx, rel)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			continue
		}
		hash := hashBytes(data)
		if prev, ok := st.Assets[rel]; ok && prev.BaseHash == hash {
			continue
		}
		err = e.retry(ctx, func() error { return client.PutAsset(ctx, rel, bytes.NewReader(data)) })
		if err != nil {
			if errors.Is(err, ErrNotSupported) {
				e.logger.Warn("asset sync skipped: remote backend does not support path writes", "path", rel)
				continue
			}
			if errors.Is(err, ErrTooLarge) {
				// A 413 is final (retryable returns false), so the asset is
				// skipped rather than retried; chunked upload is not supported.
				tooLarge := fmt.Sprintf("asset %s (%d bytes) exceeds the remote upload limit and was skipped; chunked upload is not supported", rel, len(data))
				e.setWarning(tooLarge)
				e.logger.Warn("asset too large; skipping", "path", rel, "size", len(data))
				continue
			}
			e.logger.Warn("asset push failed", "path", rel, "error", err)
			continue
		}
		_ = e.state.Mutate(func(d *stateData) {
			d.Assets[rel] = assetState{BaseHash: hash, Size: int64(len(data))}
		})
	}
}

// pullAssets downloads attachments referenced by a note that are missing
// locally. Backends without path restore (S3) are skipped with a warning.
func (e *Engine) pullAssets(ctx context.Context, client *RemoteClient, noteContent string) {
	refs := assetRefs(noteContent)
	if len(refs) == 0 {
		return
	}
	restorer, canRestore := e.assets.(core.AssetRestorer)
	for _, rel := range refs {
		if reader, err := e.assets.Open(ctx, rel); err == nil {
			reader.Close()
			continue
		}
		if !canRestore {
			e.logger.Warn("asset sync skipped: local backend cannot restore paths", "path", rel)
			continue
		}
		body, err := client.GetAsset(ctx, rel)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				e.logger.Warn("asset pull failed", "path", rel, "error", err)
			}
			continue
		}
		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			e.logger.Warn("read remote asset", "path", rel, "error", err)
			continue
		}
		if err := restorer.Restore(ctx, rel, bytes.NewReader(data)); err != nil {
			if errors.Is(err, core.ErrNotSupported) {
				e.logger.Warn("asset sync skipped: local backend cannot restore paths", "path", rel)
				continue
			}
			e.logger.Warn("restore asset", "path", rel, "error", err)
			continue
		}
		_ = e.state.Mutate(func(d *stateData) {
			d.Assets[rel] = assetState{BaseHash: hashBytes(data), Size: int64(len(data))}
		})
	}
}

// ---------------------------------------------------------------------------
// retry and status
// ---------------------------------------------------------------------------

const (
	maxAttempts = 5
	maxBackoff  = 8 * time.Second
)

func (e *Engine) retry(ctx context.Context, op func() error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = op()
		if err == nil {
			return nil
		}
		if !retryable(err) {
			return err
		}
		delay := time.Duration(1<<uint(attempt)) * 300 * time.Millisecond
		if delay > maxBackoff {
			delay = maxBackoff
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return err
}

// retryable reports whether an error is worth another attempt: transport
// failures, 5xx and 429. Auth, conflict, not-found and unsupported are final.
func retryable(err error) bool {
	switch {
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrForbidden),
		errors.Is(err, ErrConflict), errors.Is(err, ErrNotFound),
		errors.Is(err, ErrNotSupported), errors.Is(err, ErrTooLarge):
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status == http.StatusTooManyRequests
	}
	if errors.Is(err, ErrServer) {
		return true
	}
	// Transport-level failures are retryable.
	return true
}

func (e *Engine) setStatus(status string) {
	e.statusMu.Lock()
	e.status = status
	e.statusMu.Unlock()
}

func (e *Engine) setProgress(done, total int) {
	e.statusMu.Lock()
	e.progress = server.SyncProgress{Done: done, Total: total}
	e.statusMu.Unlock()
}

func (e *Engine) setError(err error) {
	e.statusMu.Lock()
	if err == nil {
		e.lastError = ""
	} else {
		e.lastError = err.Error()
	}
	e.statusMu.Unlock()
}

// setWarning records a non-fatal problem (such as a skipped oversized asset)
// that should surface in the sync status even though the sync succeeds.
func (e *Engine) setWarning(msg string) {
	e.statusMu.Lock()
	e.warning = msg
	e.statusMu.Unlock()
}

func (e *Engine) clearWarning() {
	e.statusMu.Lock()
	e.warning = ""
	e.statusMu.Unlock()
}

func (e *Engine) setConnected(ok bool) {
	e.statusMu.Lock()
	e.connected = ok
	e.statusMu.Unlock()
}

func (e *Engine) fail(err error) {
	e.setStatus(StatusError)
	e.setError(err)
	e.setConnected(false)
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// assetRefs extracts the unique vault-relative attachment paths referenced by
// Markdown content.
func assetRefs(content string) []string {
	matches := assetRefPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		rel := m[1]
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	return out
}
