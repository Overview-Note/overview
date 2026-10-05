package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

type fakeSync struct {
	status       server.SyncStatus
	configured   server.SyncConfig
	configureErr error
	ran          bool
	conflicts    []server.SyncConflict
	resolved     []string
}

func (f *fakeSync) SyncStatus(context.Context) (server.SyncStatus, error) { return f.status, nil }
func (f *fakeSync) SyncConfigure(_ context.Context, cfg server.SyncConfig) (server.SyncStatus, error) {
	f.configured = cfg
	return f.status, f.configureErr
}
func (f *fakeSync) SyncRun(context.Context) error { f.ran = true; return nil }
func (f *fakeSync) SyncConflicts(context.Context) ([]server.SyncConflict, error) {
	return f.conflicts, nil
}
func (f *fakeSync) SyncResolveConflict(_ context.Context, id, keep string) error {
	f.resolved = append(f.resolved, id+":"+keep)
	return nil
}

func newSyncServer(t *testing.T, localOnly bool, hooks server.SyncHooks) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	svc := service.New(st, ix, st, nil, nil)
	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		LocalOnly:      localOnly,
		SyncHooks:      hooks,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestSyncEndpointsHeadless404(t *testing.T) {
	ts := newSyncServer(t, false, &fakeSync{})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		resp, _ := doJSON(t, method, ts.URL+"/api/v1/desktop/sync", map[string]any{})
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s /desktop/sync headless = %d, want 404", method, resp.StatusCode)
		}
	}
	if resp, _ := doJSON(t, http.MethodPost, ts.URL+"/api/v1/desktop/sync/run", map[string]any{}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("headless run = %d, want 404", resp.StatusCode)
	}
}

func TestSyncEndpointsHiddenWithoutHooks(t *testing.T) {
	ts := newSyncServer(t, true, nil)
	if resp, _ := doJSON(t, http.MethodGet, ts.URL+"/api/v1/desktop/sync", nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("no-hooks sync = %d, want 404", resp.StatusCode)
	}
}

func TestSyncStatusAndConfigure(t *testing.T) {
	fake := &fakeSync{status: server.SyncStatus{
		Enabled: true, ServerURL: "https://notes.example.com", Direction: "both", IntervalSec: 60, Status: "idle",
	}}
	ts := newSyncServer(t, true, fake)

	resp, body := doJSON(t, http.MethodGet, ts.URL+"/api/v1/desktop/sync", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET sync = %d", resp.StatusCode)
	}
	if body["serverURL"] != "https://notes.example.com" {
		t.Errorf("status body = %v", body)
	}

	secret := "tok-do-not-leak"
	resp, body = doJSON(t, http.MethodPut, ts.URL+"/api/v1/desktop/sync", map[string]any{
		"serverURL": "https://notes.example.com", "token": secret, "enabled": true,
		"direction": "pull", "intervalSec": 30,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT sync = %d %v", resp.StatusCode, body)
	}
	if fake.configured.ServerURL == nil || *fake.configured.ServerURL != "https://notes.example.com" {
		t.Errorf("configured = %+v", fake.configured)
	}
	if fake.configured.Token == nil || *fake.configured.Token != secret {
		t.Errorf("token not forwarded: %+v", fake.configured)
	}
	if fake.configured.Direction == nil || *fake.configured.Direction != "pull" {
		t.Errorf("direction not forwarded: %+v", fake.configured)
	}
	encoded, _ := json.Marshal(body)
	if strings.Contains(string(encoded), secret) {
		t.Fatal("response echoed the token")
	}
}

func TestSyncConfigureRejectsBadDirection(t *testing.T) {
	ts := newSyncServer(t, true, &fakeSync{})
	if resp, _ := doJSON(t, http.MethodPut, ts.URL+"/api/v1/desktop/sync", map[string]any{"direction": "sideways"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad direction = %d, want 400", resp.StatusCode)
	}
}

func TestSyncRunIsAsync(t *testing.T) {
	fake := &fakeSync{}
	ts := newSyncServer(t, true, fake)
	resp, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/desktop/sync/run", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("run = %d", resp.StatusCode)
	}
	if body["started"] != true {
		t.Errorf("run body = %v", body)
	}
	if !fake.ran {
		t.Error("SyncRun was not invoked")
	}
}

func TestGetNoteRaw(t *testing.T) {
	ts := newTestServer(t)
	content := "---\nid: raw-id\ntitle: Raw\n---\n\nbody\n"
	resp, created := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]any{
		"path": "raw.md", "raw": true, "content": content, "baseVersion": "*",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw create = %d %v", resp.StatusCode, created)
	}
	version, _ := created["version"].(string)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/note?path=raw.md&raw=1", nil)
	got, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("raw get = %d", got.StatusCode)
	}
	if want := `"` + version + `"`; got.Header.Get("ETag") != want {
		t.Errorf("raw ETag = %q, want %q", got.Header.Get("ETag"), want)
	}
	raw, _ := io.ReadAll(got.Body)
	if string(raw) != content {
		t.Errorf("raw body = %q, want %q", raw, content)
	}
}

func TestSyncConflictsAndResolve(t *testing.T) {
	fake := &fakeSync{conflicts: []server.SyncConflict{{ID: "id-1", ConflictPath: "a (conflict).md"}}}
	ts := newSyncServer(t, true, fake)

	resp, body := doJSON(t, http.MethodGet, ts.URL+"/api/v1/desktop/sync/conflicts", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("conflicts = %d", resp.StatusCode)
	}
	list, ok := body["conflicts"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("conflicts body = %v", body)
	}

	if resp, _ := doJSON(t, http.MethodPost, ts.URL+"/api/v1/desktop/sync/conflicts/resolve", map[string]any{"id": "id-1", "keep": "local"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve = %d", resp.StatusCode)
	}
	if len(fake.resolved) != 1 || fake.resolved[0] != "id-1:local" {
		t.Errorf("resolved = %v", fake.resolved)
	}
	if resp, _ := doJSON(t, http.MethodPost, ts.URL+"/api/v1/desktop/sync/conflicts/resolve", map[string]any{"id": "id-1", "keep": "maybe"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad keep = %d, want 400", resp.StatusCode)
	}
}
