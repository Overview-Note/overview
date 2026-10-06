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

// TestSyncRealTimeAndFolders runs two real app instances and verifies that a
// server-side note change reaches the desktop over the change stream within
// seconds, and that empty folders are created and deleted in both directions.
func TestSyncRealTimeAndFolders(t *testing.T) {
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

	// A long poll interval so convergence must come from the change stream or
	// the local watcher, never the poll timer.
	cfgBody, _ := json.Marshal(map[string]any{
		"serverURL": "http://" + serverAddr, "token": "ignored", "enabled": true, "direction": "both", "intervalSec": 3600,
	})
	if code := doJSON(t, http.MethodPut, "http://"+desktopAddr+"/api/v1/desktop/sync", cfgBody); code != http.StatusOK {
		t.Fatalf("configure sync = %d", code)
	}

	// Server-side note edit converges on the desktop through SSE.
	if code := doJSON(t, http.MethodPut, "http://"+serverAddr+"/api/v1/note", []byte(`{"path":"live.md","body":"from-server","baseVersion":"*"}`)); code != http.StatusOK {
		t.Fatalf("create note = %d", code)
	}
	waitFor(t, 10*time.Second, func() bool {
		_, err := os.Stat(filepath.Join(desktopCfg.NotesDir, "live.md"))
		return err == nil
	}, "desktop did not converge the server note over SSE")

	// Server creates an empty folder; pull it with an explicit run.
	if code := doJSON(t, http.MethodPost, "http://"+serverAddr+"/api/v1/folder", []byte(`{"path":"empty"}`)); code != http.StatusOK {
		t.Fatalf("create folder = %d", code)
	}
	if code := requestStatus(t, http.MethodPost, "http://"+desktopAddr+"/api/v1/desktop/sync/run"); code != http.StatusOK {
		t.Fatalf("sync run = %d", code)
	}
	waitFor(t, 10*time.Second, func() bool { return isDir(filepath.Join(desktopCfg.NotesDir, "empty")) }, "desktop did not pull the empty folder")

	// Server deletes the empty folder; the desktop removes it over SSE.
	if code := requestStatus(t, http.MethodDelete, "http://"+serverAddr+"/api/v1/note?path=empty"); code != http.StatusOK {
		t.Fatalf("delete folder = %d", code)
	}
	waitFor(t, 10*time.Second, func() bool { return !isDir(filepath.Join(desktopCfg.NotesDir, "empty")) }, "desktop did not remove the deleted empty folder over SSE")

	// Desktop creates an empty folder; the watcher pushes it to the server.
	if err := os.Mkdir(filepath.Join(desktopCfg.NotesDir, "local-empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool { return serverHasFolder(t, serverAddr, "local-empty") }, "server did not receive the desktop folder")

	// Desktop deletes the empty folder; the server removes it.
	if err := os.Remove(filepath.Join(desktopCfg.NotesDir, "local-empty")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool { return !serverHasFolder(t, serverAddr, "local-empty") }, "server kept the desktop-deleted folder")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func doJSON(t *testing.T, method, url string, body []byte) int {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func serverHasFolder(t *testing.T, addr, folder string) bool {
	t.Helper()
	manifest := getJSON(t, "http://"+addr+"/api/v1/sync/manifest")
	folders, _ := manifest["folders"].([]any)
	for _, f := range folders {
		if f == folder {
			return true
		}
	}
	return false
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
