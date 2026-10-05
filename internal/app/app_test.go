package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig(t *testing.T, addr string) config.Config {
	t.Helper()
	t.Setenv("OVERVIEW_S3_BUCKET", "")
	cfg := config.Load()
	cfg = cfg.WithDataDir(t.TempDir())
	cfg.Desktop = true
	cfg.AuthMode = "none"
	cfg.Addr = addr
	cfg.EnableMCP = false
	cfg.EnableDAV = false
	cfg.Render = false
	return cfg
}

func TestInstanceLockIsExclusive(t *testing.T) {
	dir := t.TempDir()

	first, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := AcquireInstanceLock(dir); err == nil {
		t.Fatal("second acquire succeeded, want failure")
	}
	if err := first.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}

	again, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	if err := again.Release(); err != nil {
		t.Fatalf("release again: %v", err)
	}
}

func TestPortFile(t *testing.T) {
	dir := t.TempDir()
	path, err := writePortFile(dir, "127.0.0.1:5230")
	if err != nil {
		t.Fatalf("writePortFile: %v", err)
	}
	if filepath.Base(path) != portFileName {
		t.Errorf("port file name = %q, want %q", filepath.Base(path), portFileName)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read port file: %v", err)
	}
	var info portInfo
	if err := json.Unmarshal(b, &info); err != nil {
		t.Fatalf("parse port file: %v", err)
	}
	if info.Addr != "127.0.0.1:5230" || info.PID != os.Getpid() {
		t.Errorf("port file = %+v, want addr 127.0.0.1:5230 pid %d", info, os.Getpid())
	}

	if err := removePortFile(path); err != nil {
		t.Fatalf("removePortFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("port file still present: %v", err)
	}
}

func TestNewEnforcesSingleInstance(t *testing.T) {
	cfg := testConfig(t, "127.0.0.1:0")
	a, err := New(cfg, discardLogger())
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	defer func() { _ = a.Stop(context.Background()) }()

	if _, err := New(cfg, discardLogger()); err == nil {
		t.Fatal("second New succeeded, want single-instance error")
	}
}

func TestResolveOrImportKeepsVaultPath(t *testing.T) {
	cfg := testConfig(t, "127.0.0.1:0")
	a, err := New(cfg, discardLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Stop(context.Background()) }()

	if err := os.WriteFile(filepath.Join(cfg.NotesDir, "inner.md"), []byte("# inner"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := a.ResolveOrImport(context.Background(), filepath.Join(cfg.NotesDir, "inner.md"))
	if err != nil {
		t.Fatalf("ResolveOrImport: %v", err)
	}
	if got != "inner.md" {
		t.Errorf("rel = %q, want inner.md", got)
	}
}

func TestResolveOrImportCopiesExternalFile(t *testing.T) {
	cfg := testConfig(t, "127.0.0.1:0")
	a, err := New(cfg, discardLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Stop(context.Background()) }()

	src := filepath.Join(t.TempDir(), "external note.md")
	if err := os.WriteFile(src, []byte("# imported"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := a.ResolveOrImport(context.Background(), src)
	if err != nil {
		t.Fatalf("ResolveOrImport: %v", err)
	}
	if got != "external note.md" {
		t.Errorf("rel = %q, want external note.md", got)
	}
	copied := filepath.Join(cfg.NotesDir, "external note.md")
	if _, err := os.Stat(copied); err != nil {
		t.Fatalf("imported file missing: %v", err)
	}
	if _, err := a.svc.GetNote(context.Background(), got); err != nil {
		t.Errorf("GetNote(%q): %v", got, err)
	}

	again, err := a.ResolveOrImport(context.Background(), src)
	if err != nil {
		t.Fatalf("second ResolveOrImport: %v", err)
	}
	if again != "external note (1).md" {
		t.Errorf("collision rel = %q, want external note (1).md", again)
	}
}

func TestStartFallsBackToRandomPort(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	cfg := testConfig(t, occupied.Addr().String())
	a, err := New(cfg, discardLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Stop(context.Background()) }()

	addr, err := a.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if addr == occupied.Addr().String() {
		t.Fatalf("expected a fallback port, still on %s", addr)
	}

	b, err := os.ReadFile(filepath.Join(cfg.DataDir, portFileName))
	if err != nil {
		t.Fatalf("read port file: %v", err)
	}
	var info portInfo
	if err := json.Unmarshal(b, &info); err != nil {
		t.Fatalf("parse port file: %v", err)
	}
	if info.Addr != addr {
		t.Errorf("port file addr = %q, want %q", info.Addr, addr)
	}
	if info.PID != os.Getpid() {
		t.Errorf("port file pid = %d, want %d", info.PID, os.Getpid())
	}

	// Health is available immediately, before the background reindex finishes.
	resp, err := http.Get("http://" + addr + "/api/v1/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}

	// MCP and WebDAV are disabled by default on desktop, so neither endpoint is
	// mounted. Assert on the absence of a protocol response rather than the SPA
	// fallback: the embedded frontend is not built in CI, where /mcp falls
	// through to a plain 404 instead of index.html.
	status, contentType, body := postResponse(t, "http://"+addr+"/mcp")
	if strings.HasPrefix(contentType, "application/json") || status == http.StatusAccepted || strings.Contains(body, "jsonrpc") {
		t.Errorf("disabled /mcp answered as MCP: status = %d, content-type = %q, body = %q", status, contentType, body)
	}
	if code := propfindStatus(t, "http://"+addr+"/dav/"); code == 207 {
		t.Error("disabled /dav/ answered a WebDAV PROPFIND")
	}
}

func TestEnabledMCPAndDAVRequireToken(t *testing.T) {
	cfg := testConfig(t, "127.0.0.1:0")
	cfg.EnableMCP = true
	cfg.EnableDAV = true
	cfg.MCPToken = "secret-token"

	a, err := New(cfg, discardLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Stop(context.Background()) }()

	addr, err := a.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	base := "http://" + addr

	// Anonymous MCP is refused even though auth is disabled.
	if code := postStatus(t, base+"/mcp", ""); code != http.StatusUnauthorized {
		t.Errorf("anonymous /mcp = %d, want 401", code)
	}
	if code := postStatus(t, base+"/mcp", "Bearer secret-token"); code == http.StatusUnauthorized {
		t.Errorf("token /mcp = %d, want authorized", code)
	}

	// Anonymous WebDAV is refused; the token is accepted as a Basic password.
	if code := propfindStatus(t, base+"/dav/"); code != http.StatusUnauthorized {
		t.Errorf("anonymous /dav/ = %d, want 401", code)
	}
	req, _ := http.NewRequest("PROPFIND", base+"/dav/", nil)
	req.SetBasicAuth("overview", "secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("token-authenticated /dav/ = 401, want authorized")
	}
}

func postStatus(t *testing.T, url, authorization string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func postResponse(t *testing.T, url string) (int, string, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(body)
}

func propfindStatus(t *testing.T, url string) int {
	t.Helper()
	req, _ := http.NewRequest("PROPFIND", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
