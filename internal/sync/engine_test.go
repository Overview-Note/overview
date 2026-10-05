package sync

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/markdown"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

// testVault is a full local Overview instance backed by a temporary directory.
type testVault struct {
	t     *testing.T
	dir   string
	store *store.Store
	index *index.Index
	svc   *service.Service
}

func newTestVault(t *testing.T) *testVault {
	t.Helper()
	dir := t.TempDir()
	notesDir := filepath.Join(dir, "notes")
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := store.New(notesDir, assetsDir)
	ix, err := index.Open(filepath.Join(dir, "overview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return &testVault{t: t, dir: dir, store: st, index: ix, svc: service.New(st, ix, st, nil, nil)}
}

func (v *testVault) ctx() context.Context { return context.Background() }

// writeNote writes a note with an explicit frontmatter id and reindexes it.
func (v *testVault) writeNote(rel, id, body string) {
	v.t.Helper()
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	doc := markdown.Document{Meta: markdown.Frontmatter{ID: id, Title: name}, Body: body}
	if err := v.store.WriteRaw(v.ctx(), rel, []byte(doc.String())); err != nil {
		v.t.Fatalf("write %s: %v", rel, err)
	}
	if err := v.svc.ReindexPath(v.ctx(), rel); err != nil {
		v.t.Fatalf("reindex %s: %v", rel, err)
	}
}

func (v *testVault) body(rel string) string {
	v.t.Helper()
	note, err := v.store.Read(v.ctx(), rel)
	if err != nil {
		v.t.Fatalf("read %s: %v", rel, err)
	}
	return note.Body
}

func (v *testVault) have(rel string) bool {
	_, err := v.store.Read(v.ctx(), rel)
	return err == nil
}

func newRemoteServer(t *testing.T, v *testVault) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(server.New(v.svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Version:        "test",
		Logger:         discardLogger(),
	}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

// newEngine builds and configures an engine against a remote URL.
func newEngine(t *testing.T, local *testVault, remoteURL string, direction Direction) *Engine {
	t.Helper()
	return newEngineInterval(t, local, remoteURL, direction, 1)
}

// newEngineInterval is newEngine with an explicit poll interval.
func newEngineInterval(t *testing.T, local *testVault, remoteURL string, direction Direction, interval int) *Engine {
	t.Helper()
	eng, err := NewEngine(Options{
		DataDir: t.TempDir(),
		Repo:    local.store,
		Raw:     local.store,
		Assets:  local.store,
		Logger:  discardLogger(),
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	dir := string(direction)
	if _, err := eng.SyncConfigure(context.Background(), server.SyncConfig{
		ServerURL:   strPtr(remoteURL),
		Enabled:     boolPtr(true),
		Direction:   &dir,
		IntervalSec: intPtr(interval),
	}); err != nil {
		t.Fatalf("SyncConfigure: %v", err)
	}
	return eng
}

// newCountingRemoteServer wraps a remote vault in an httptest server that
// counts manifest requests, so tests can observe how many syncs ran.
func newCountingRemoteServer(t *testing.T, v *testVault) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	count := 0
	handler := server.New(v.svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Version:        "test",
		Logger:         discardLogger(),
	}).Handler()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/sync/manifest" {
			mu.Lock()
			count++
			mu.Unlock()
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, func() int {
		mu.Lock()
		defer mu.Unlock()
		return count
	}
}

func mustSync(t *testing.T, eng *Engine) {
	t.Helper()
	if err := eng.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
}

func TestSyncCreateLocalPushes(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("a.md", "id-a", "hello")
	mustSync(t, eng)

	if !remote.have("a.md") {
		t.Fatal("remote missing pushed note a.md")
	}
	if got := remote.body("a.md"); got != "hello" {
		t.Errorf("remote body = %q, want hello", got)
	}
}

func TestSyncCreateRemotePulls(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	remote.writeNote("b.md", "id-b", "remote")
	mustSync(t, eng)

	if !local.have("b.md") {
		t.Fatal("local missing pulled note b.md")
	}
	if got := local.body("b.md"); got != "remote" {
		t.Errorf("local body = %q, want remote", got)
	}
}

func TestSyncConvergesOnEdit(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("c.md", "id-c", "v1")
	mustSync(t, eng)

	local.writeNote("c.md", "id-c", "v2-local")
	mustSync(t, eng)
	if got := remote.body("c.md"); got != "v2-local" {
		t.Errorf("after local edit remote = %q, want v2-local", got)
	}

	remote.writeNote("c.md", "id-c", "v3-remote")
	mustSync(t, eng)
	if got := local.body("c.md"); got != "v3-remote" {
		t.Errorf("after remote edit local = %q, want v3-remote", got)
	}
}

func TestSyncConflictProducesCopy(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("d.md", "id-d", "base")
	mustSync(t, eng)

	local.writeNote("d.md", "id-d", "local-change")
	remote.writeNote("d.md", "id-d", "remote-change")
	mustSync(t, eng)

	// The remote original wins the original path.
	if got := local.body("d.md"); got != "remote-change" {
		t.Errorf("original path = %q, want remote-change", got)
	}

	conflicts, err := eng.SyncConflicts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want 1", conflicts)
	}
	c := conflicts[0]
	if c.ID != "id-d" || !strings.Contains(c.ConflictPath, "conflict-") {
		t.Fatalf("conflict = %+v", c)
	}
	if !local.have(c.ConflictPath) {
		t.Fatalf("local conflict copy %q missing", c.ConflictPath)
	}
	if got := local.body(c.ConflictPath); got != "local-change" {
		t.Errorf("conflict copy body = %q, want local-change", got)
	}
	if !remote.have(c.ConflictPath) {
		t.Errorf("remote conflict copy %q missing", c.ConflictPath)
	}
}

func TestSyncResolveConflict(t *testing.T) {
	for _, keep := range []string{"local", "remote"} {
		t.Run(keep, func(t *testing.T) {
			local := newTestVault(t)
			remote := newTestVault(t)
			ts := newRemoteServer(t, remote)
			eng := newEngine(t, local, ts.URL, DirectionBoth)

			local.writeNote("d.md", "id-d", "base")
			mustSync(t, eng)
			local.writeNote("d.md", "id-d", "local-change")
			remote.writeNote("d.md", "id-d", "remote-change")
			mustSync(t, eng)

			conflicts, _ := eng.SyncConflicts(context.Background())
			cp := conflicts[0].ConflictPath

			if err := eng.SyncResolveConflict(context.Background(), "id-d", keep); err != nil {
				t.Fatalf("resolve %s: %v", keep, err)
			}
			remaining, _ := eng.SyncConflicts(context.Background())
			if len(remaining) != 0 {
				t.Errorf("conflicts after resolve = %+v", remaining)
			}
			if local.have(cp) {
				t.Errorf("conflict copy %q should be gone", cp)
			}
			mustSync(t, eng)
			if keep == "local" {
				if got := local.body("d.md"); got != "local-change" {
					t.Errorf("local resolve body = %q", got)
				}
				if got := remote.body("d.md"); got != "local-change" {
					t.Errorf("remote after local resolve = %q", got)
				}
			} else {
				if got := remote.body("d.md"); got != "remote-change" {
					t.Errorf("remote resolve body = %q", got)
				}
			}
		})
	}
}

func TestSyncDeletePropagates(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("e.md", "id-e", "gone")
	mustSync(t, eng)
	if !remote.have("e.md") {
		t.Fatal("remote missing e.md after push")
	}

	if err := local.svc.Delete(local.ctx(), "e.md"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)
	if remote.have("e.md") {
		t.Error("remote e.md survived a local delete")
	}

	// The reverse direction.
	remote.writeNote("f.md", "id-f", "reverse")
	mustSync(t, eng)
	if !local.have("f.md") {
		t.Fatal("local missing f.md after pull")
	}
	if err := remote.svc.Delete(remote.ctx(), "f.md"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)
	if local.have("f.md") {
		t.Error("local f.md survived a remote delete")
	}
}

func TestSyncMovePropagates(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("g.md", "id-g", "movable")
	mustSync(t, eng)

	if err := local.svc.Move(local.ctx(), "g.md", "g2.md"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)
	if remote.have("g.md") || !remote.have("g2.md") {
		t.Errorf("remote move failed: g=%v g2=%v", remote.have("g.md"), remote.have("g2.md"))
	}

	if err := remote.svc.Move(remote.ctx(), "g2.md", "g3.md"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)
	if local.have("g2.md") || !local.have("g3.md") {
		t.Errorf("local move failed: g2=%v g3=%v", local.have("g2.md"), local.have("g3.md"))
	}
}

func TestSyncAssignsMissingID(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	if err := local.store.WriteRaw(local.ctx(), "noid.md", []byte("# no id\n\nbody\n")); err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)

	note, err := local.store.Read(local.ctx(), "noid.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.ID == "" {
		t.Fatal("local note did not get an id")
	}
	remoteNote, err := remote.store.Read(remote.ctx(), "noid.md")
	if err != nil {
		t.Fatalf("remote missing noid.md: %v", err)
	}
	if remoteNote.ID != note.ID {
		t.Errorf("remote id = %q, want %q", remoteNote.ID, note.ID)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("h.md", "id-h", "stable")
	mustSync(t, eng)

	first, _, err := eng.currentClient().Manifest(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)
	mustSync(t, eng)
	second, _, err := eng.currentClient().Manifest(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ETag != second.ETag {
		t.Errorf("manifest changed on a no-op sync: %s -> %s", first.ETag, second.ETag)
	}
	conflicts, _ := eng.SyncConflicts(context.Background())
	if len(conflicts) != 0 {
		t.Errorf("unexpected conflicts: %+v", conflicts)
	}
}

func TestSyncDirectionPullOnly(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionPull)

	local.writeNote("local.md", "id-local", "local-only")
	remote.writeNote("remote.md", "id-remote", "remote-only")
	mustSync(t, eng)

	if !local.have("remote.md") {
		t.Error("pull-only did not pull remote.md")
	}
	if remote.have("local.md") {
		t.Error("pull-only pushed local.md")
	}
}

func TestSyncDirectionPushOnly(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionPush)

	local.writeNote("local.md", "id-local", "local-only")
	remote.writeNote("remote.md", "id-remote", "remote-only")
	mustSync(t, eng)

	if !remote.have("local.md") {
		t.Error("push-only did not push local.md")
	}
	if local.have("remote.md") {
		t.Error("push-only pulled remote.md")
	}
}

func TestSyncPullDoesNotUploadLocalEdit(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionPull)

	remote.writeNote("p.md", "id-p", "base")
	mustSync(t, eng)
	if got := local.body("p.md"); got != "base" {
		t.Fatalf("initial pull body = %q, want base", got)
	}

	local.writeNote("p.md", "id-p", "local-edit")
	mustSync(t, eng)
	if got := remote.body("p.md"); got != "base" {
		t.Errorf("pull direction uploaded a local edit: remote = %q, want base", got)
	}

	remote.writeNote("p.md", "id-p", "remote-edit")
	mustSync(t, eng)
	if got := local.body("p.md"); got != "remote-edit" {
		t.Errorf("pull direction did not download a remote edit: local = %q, want remote-edit", got)
	}
}

func TestSyncPushDoesNotDownloadRemoteEdit(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionPush)

	local.writeNote("q.md", "id-q", "base")
	mustSync(t, eng)
	if got := remote.body("q.md"); got != "base" {
		t.Fatalf("initial push body = %q, want base", got)
	}

	remote.writeNote("q.md", "id-q", "remote-edit")
	mustSync(t, eng)
	if got := local.body("q.md"); got != "base" {
		t.Errorf("push direction downloaded a remote edit: local = %q, want base", got)
	}

	local.writeNote("q.md", "id-q", "local-edit")
	mustSync(t, eng)
	if got := remote.body("q.md"); got != "local-edit" {
		t.Errorf("push direction did not upload a local edit: remote = %q, want local-edit", got)
	}
}

func TestSyncBothPropagatesBothWays(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	local.writeNote("w.md", "id-w", "base")
	mustSync(t, eng)

	local.writeNote("w.md", "id-w", "from-local")
	mustSync(t, eng)
	if got := remote.body("w.md"); got != "from-local" {
		t.Errorf("both did not push local edit: remote = %q", got)
	}

	remote.writeNote("w.md", "id-w", "from-remote")
	mustSync(t, eng)
	if got := local.body("w.md"); got != "from-remote" {
		t.Errorf("both did not pull remote edit: local = %q", got)
	}
}

func TestSyncPullDoesNotDeleteRemote(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionPull)

	remote.writeNote("del.md", "id-del", "keep")
	mustSync(t, eng)
	if !local.have("del.md") {
		t.Fatal("pull did not fetch del.md")
	}
	if err := local.svc.Delete(local.ctx(), "del.md"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, eng)
	if !remote.have("del.md") {
		t.Error("pull direction propagated a local delete to the server")
	}
}

func TestSyncRunBypassesThrottle(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts, manifests := newCountingRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	before := manifests()
	for i := 0; i < 2; i++ {
		if err := eng.SyncRun(context.Background()); err != nil {
			t.Fatalf("SyncRun: %v", err)
		}
	}
	waitForCount(t, 5*time.Second, manifests, before+2, "explicit runs were throttled")
}

func TestSyncRunForcesThroughLoopThrottle(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts, manifests := newCountingRemoteServer(t, remote)
	eng := newEngineInterval(t, local, ts.URL, DirectionBoth, 3600)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Start(ctx)
	defer func() { _ = eng.Stop(context.Background()) }()

	waitForCount(t, 5*time.Second, manifests, 1, "loop initial sync")
	before := manifests()
	for i := 0; i < 2; i++ {
		if err := eng.SyncRun(context.Background()); err != nil {
			t.Fatalf("SyncRun: %v", err)
		}
	}
	waitForCount(t, 5*time.Second, manifests, before+2, "loop did not run forced syncs")
}

func waitForCount(t *testing.T, timeout time.Duration, count func() int, want int, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if count() >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s: got %d, want >= %d", msg, count(), want)
}

func TestSyncAssetsByPath(t *testing.T) {
	local := newTestVault(t)
	remote := newTestVault(t)
	ts := newRemoteServer(t, remote)
	eng := newEngine(t, local, ts.URL, DirectionBoth)

	data := []byte("PNGDATA")
	if err := local.store.Restore(local.ctx(), "attachments/pic.png", strings.NewReader(string(data))); err != nil {
		t.Fatal(err)
	}
	local.writeNote("with-asset.md", "id-asset", "![pic](/assets/attachments/pic.png)\n")
	mustSync(t, eng)

	reader, err := remote.store.Open(remote.ctx(), "attachments/pic.png")
	if err != nil {
		t.Fatalf("remote asset missing: %v", err)
	}
	got, _ := io.ReadAll(reader)
	reader.Close()
	if string(got) != string(data) {
		t.Errorf("remote asset = %q, want %q", got, data)
	}

	// The reverse direction: a remote note with an attachment pulls it down.
	remoteData := []byte("JPEGDATA")
	if err := remote.store.Restore(remote.ctx(), "media/photo.jpg", strings.NewReader(string(remoteData))); err != nil {
		t.Fatal(err)
	}
	remote.writeNote("remote-asset.md", "id-asset2", "![p](/assets/media/photo.jpg)\n")
	mustSync(t, eng)

	r2, err := local.store.Open(local.ctx(), "media/photo.jpg")
	if err != nil {
		t.Fatalf("local asset missing: %v", err)
	}
	got2, _ := io.ReadAll(r2)
	r2.Close()
	if string(got2) != string(remoteData) {
		t.Errorf("local asset = %q, want %q", got2, remoteData)
	}
}

func TestStateDoesNotPersistToken(t *testing.T) {
	dir := t.TempDir()
	vault := newTestVault(t)
	eng, err := NewEngine(Options{
		DataDir: dir,
		Repo:    vault.store,
		Raw:     vault.store,
		Assets:  vault.store,
		Logger:  discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteToken(dir, "super-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.SyncConfigure(context.Background(), server.SyncConfig{Token: strPtr("super-secret")}); err != nil {
		t.Fatalf("configure: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "sync.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret") {
		t.Fatal("token leaked into sync.json")
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	token, err := ReadToken(dir)
	if err != nil || token != "super-secret" {
		t.Fatalf("token = %q, %v", token, err)
	}
	info, err := os.Stat(tokenPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Errorf("token file permissions = %v, want owner-only", info.Mode().Perm())
	}
}

func TestAssetRefs(t *testing.T) {
	content := "see /assets/a/b.png and assets/c.png and /api/v1/assets/d.png and /assets/a/b.png"
	refs := assetRefs(content)
	want := map[string]bool{"a/b.png": true, "c.png": true, "d.png": true}
	if len(refs) != len(want) {
		t.Fatalf("refs = %v, want %v", refs, want)
	}
	for _, r := range refs {
		if !want[r] {
			t.Errorf("unexpected ref %q", r)
		}
	}
}

func TestConflictPathFor(t *testing.T) {
	got := conflictPathFor("dir/note.md", "device-123456789")
	if !strings.HasPrefix(got, "dir/note (conflict-device-1-") || !strings.HasSuffix(got, ").md") {
		t.Errorf("conflictPathFor = %q", got)
	}
}
