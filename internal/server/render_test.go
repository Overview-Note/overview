package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

func newRenderServer(t *testing.T) *httptest.Server {
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
		Render:         true,
		MaxUploadBytes: 1 << 20,
	}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

// TestRenderModeHidesPrivateEndpoints verifies that a public documentation site
// only exposes the whitelisted read endpoints and hides private ones.
func TestRenderModeHidesPrivateEndpoints(t *testing.T) {
	ts := newRenderServer(t)
	cases := []struct {
		path string
		want int
	}{
		{"/api/v1/health", http.StatusOK},
		{"/api/v1/public/notes", http.StatusOK},
		{"/api/v1/tree", http.StatusOK},
		{"/api/v1/history?path=x.md", http.StatusNotFound},
		{"/api/v1/history/revision?path=x.md&id=y", http.StatusNotFound},
		{"/api/v1/trash", http.StatusNotFound},
		{"/api/v1/assets/orphans", http.StatusNotFound},
		{"/api/v1/search?q=x", http.StatusNotFound},
		{"/api/v1/settings/ai", http.StatusNotFound},
	}
	for _, c := range cases {
		resp, err := http.Get(ts.URL + c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s: got %d, want %d", c.path, resp.StatusCode, c.want)
		}
	}
}

// TestRenderModeRejectsWrites ensures state-changing requests are refused.
func TestRenderModeRejectsWrites(t *testing.T) {
	ts := newRenderServer(t)
	resp, err := http.Post(ts.URL+"/api/v1/note", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /note: got %d, want 405", resp.StatusCode)
	}
}
