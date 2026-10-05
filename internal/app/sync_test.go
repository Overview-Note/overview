package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	overviewsync "github.com/Overview-Note/overview/internal/sync"
)

// TestSyncEngineLifecycleAndWatcher wires a real sync engine into a desktop
// app, points it at a second app instance and verifies the initial
// configuration, watcher-driven push and status endpoint.
func TestSyncEngineLifecycleAndWatcher(t *testing.T) {
	ctx := context.Background()

	serverCfg := testConfig(t, "127.0.0.1:0")
	serverCfg.Desktop = false
	serverApp, err := New(serverCfg, discardLogger())
	if err != nil {
		t.Fatalf("server New: %v", err)
	}
	t.Cleanup(func() { _ = serverApp.Stop(ctx) })
	serverAddr, err := serverApp.Start(ctx)
	if err != nil {
		t.Fatalf("server Start: %v", err)
	}

	desktopCfg := testConfig(t, "127.0.0.1:0")
	desktopCfg.Desktop = true
	desktopApp, err := NewWithOptions(desktopCfg, discardLogger(), Options{
		SyncFactory: func(deps SyncDeps) (SyncEngine, error) {
			return overviewsync.NewEngine(overviewsync.Options{
				DataDir: deps.DataDir,
				Repo:    deps.Repo,
				Raw:     deps.Raw,
				Assets:  deps.Assets,
				Logger:  deps.Logger,
			})
		},
	})
	if err != nil {
		t.Fatalf("desktop New: %v", err)
	}
	t.Cleanup(func() { _ = desktopApp.Stop(ctx) })
	desktopAddr, err := desktopApp.Start(ctx)
	if err != nil {
		t.Fatalf("desktop Start: %v", err)
	}

	base := "http://" + desktopAddr
	// The sync endpoints are unavailable headless and available in desktop mode.
	if code := requestStatus(t, http.MethodGet, "http://"+serverAddr+"/api/v1/desktop/sync"); code != http.StatusNotFound {
		t.Fatalf("headless sync status = %d, want 404", code)
	}
	token := "ignored-token"
	cfgBody, _ := json.Marshal(map[string]any{
		"serverURL": "http://" + serverAddr, "token": token, "enabled": true, "direction": "both", "intervalSec": 1,
	})
	req, err := http.NewRequest(http.MethodPut, base+"/api/v1/desktop/sync", bytes.NewReader(cfgBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("configure sync: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("configure sync = %d %s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte(token)) {
		t.Fatal("sync status echoed the token")
	}

	// A file dropped into the vault is pushed shortly by the watcher.
	if err := os.WriteFile(filepath.Join(desktopCfg.NotesDir, "watched.md"), []byte("# watched\n\nhello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 8*time.Second, func() bool {
		return requestStatus(t, http.MethodGet, "http://"+serverAddr+"/api/v1/note?path=watched.md") == http.StatusOK
	}, "watcher did not push the note to the server")

	// The status endpoint reports the configured server and a healthy state.
	status := getJSON(t, base+"/api/v1/desktop/sync")
	if status["serverURL"] != "http://"+serverAddr {
		t.Errorf("status serverURL = %v", status["serverURL"])
	}
	if status["enabled"] != true {
		t.Errorf("status enabled = %v", status["enabled"])
	}
}

func requestStatus(t *testing.T, method, url string) int {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.(interface {
		Read([]byte) (int, error)
	}))
	return buf.Bytes(), err
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal(msg)
}
