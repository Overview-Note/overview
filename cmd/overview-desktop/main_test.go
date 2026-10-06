//go:build windows || darwin || desktop

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/config"
)

func TestParseFlags(t *testing.T) {
	t.Setenv("OVERVIEW_DATA_DIR", "")

	base := config.Config{Addr: "127.0.0.1:5230", AuthMode: "none", DataDir: "/base"}
	flags := parseFlags([]string{"--data-dir", "/vault", "--addr=127.0.0.1:6000", "--auth", "multi"}, base)

	if !flags.explicitDataDir {
		t.Fatal("explicitDataDir = false, want true")
	}
	if flags.cfg.DataDir != "/vault" {
		t.Errorf("DataDir = %q, want /vault", flags.cfg.DataDir)
	}
	if flags.cfg.NotesDir != filepath.Join("/vault", "notes") {
		t.Errorf("NotesDir = %q, want derived from DataDir", flags.cfg.NotesDir)
	}
	if flags.cfg.Addr != "127.0.0.1:6000" {
		t.Errorf("Addr = %q", flags.cfg.Addr)
	}
	if flags.cfg.AuthMode != "multi" {
		t.Errorf("AuthMode = %q", flags.cfg.AuthMode)
	}
}

func TestParseFlagsShortAndEqualsForm(t *testing.T) {
	t.Setenv("OVERVIEW_DATA_DIR", "")

	base := config.Config{Addr: "127.0.0.1:5230", AuthMode: "none", DataDir: "/base"}
	flags := parseFlags([]string{"-C", "/vault", "--data-dir=/other", "--auth=none"}, base)

	if flags.cfg.DataDir != "/other" {
		t.Errorf("DataDir = %q, want last --data-dir wins", flags.cfg.DataDir)
	}
	if flags.cfg.AuthMode != "none" {
		t.Errorf("AuthMode = %q", flags.cfg.AuthMode)
	}
}

func TestParseFlagsExplicitFromEnv(t *testing.T) {
	t.Setenv("OVERVIEW_DATA_DIR", "/from-env")

	flags := parseFlags(nil, config.Config{DataDir: "/from-env"})
	if !flags.explicitDataDir {
		t.Fatal("explicitDataDir = false, want true when OVERVIEW_DATA_DIR is set")
	}
}

func TestNeedsBootstrap(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if !needsBootstrap(missing) {
		t.Error("missing directory should require bootstrap")
	}

	empty := t.TempDir()
	if !needsBootstrap(empty) {
		t.Error("empty directory should require bootstrap")
	}

	if err := os.WriteFile(filepath.Join(empty, "overview.db"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if needsBootstrap(empty) {
		t.Error("non-empty directory should not require bootstrap")
	}
}

func TestChooseDataDirExplicitSkipsOnboarding(t *testing.T) {
	called := false
	got, persist := chooseDataDir(true, "/explicit", config.DesktopConfig{DataDir: "/cfg"}, "/default", func(string) string {
		called = true
		return "/onboard"
	})
	if got != "/explicit" {
		t.Errorf("got %q, want /explicit", got)
	}
	if persist {
		t.Error("persist = true, want false for explicit data dir")
	}
	if called {
		t.Error("onboarding ran for an explicit data dir")
	}
}

func TestChooseDataDirUsesPersistedConfig(t *testing.T) {
	called := false
	got, persist := chooseDataDir(false, "/default", config.DesktopConfig{DataDir: "/cfg"}, "/default", func(string) string {
		called = true
		return "/onboard"
	})
	if got != "/cfg" {
		t.Errorf("got %q, want /cfg", got)
	}
	if persist {
		t.Error("persist = true, want false when a config already exists")
	}
	if called {
		t.Error("onboarding ran despite a persisted config")
	}
}

func TestChooseDataDirNonEmptyDefaultSkipsOnboarding(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overview.db"), []byte("x"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	called := false
	got, persist := chooseDataDir(false, dir, config.DesktopConfig{}, dir, func(string) string {
		called = true
		return "/onboard"
	})
	if got != dir || !persist {
		t.Errorf("got (%q, %v), want (%q, true)", got, persist, dir)
	}
	if called {
		t.Error("onboarding ran for a non-empty default directory")
	}
}

func TestChooseDataDirOnboarding(t *testing.T) {
	empty := t.TempDir()
	got, persist := chooseDataDir(false, empty, config.DesktopConfig{}, empty, func(def string) string {
		if def != empty {
			t.Errorf("onboarding def = %q, want %q", def, empty)
		}
		return "/chosen"
	})
	if got != "/chosen" || !persist {
		t.Errorf("got (%q, %v), want (/chosen, true)", got, persist)
	}
}

func TestChooseDataDirOnboardingCancelFallsBackToDefault(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	got, persist := chooseDataDir(false, missing, config.DesktopConfig{}, missing, func(string) string {
		return ""
	})
	if got != missing || !persist {
		t.Errorf("got (%q, %v), want (%q, true)", got, persist, missing)
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("processAlive(self) = false, want true")
	}
	if processAlive(1 << 30) {
		t.Error("processAlive(unused pid) = true, want false")
	}
}

func TestWaitForPredecessorExitsWhenGone(t *testing.T) {
	t.Setenv(restartWaitPIDEnv, strconv.Itoa(1<<30))
	done := make(chan struct{})
	go func() {
		waitForPredecessor()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("waitForPredecessor blocked on a non-existent process")
	}
}

func TestWaitForPredecessorNoEnv(t *testing.T) {
	t.Setenv(restartWaitPIDEnv, "")
	done := make(chan struct{})
	go func() {
		waitForPredecessor()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("waitForPredecessor blocked without the env var set")
	}
}

func TestRestartEnvDropsDataDirWhenImplicit(t *testing.T) {
	t.Setenv("OVERVIEW_DATA_DIR", "/old")
	for _, kv := range restartEnv(false, "/old") {
		if strings.HasPrefix(kv, "OVERVIEW_DATA_DIR=") {
			t.Fatal("restartEnv kept OVERVIEW_DATA_DIR for an implicit data dir")
		}
	}
}

func TestRestartEnvKeepsDataDirWhenExplicit(t *testing.T) {
	t.Setenv("OVERVIEW_DATA_DIR", "/fixed")
	found := false
	for _, kv := range restartEnv(true, "/fixed") {
		if kv == "OVERVIEW_DATA_DIR=/fixed" {
			found = true
		}
	}
	if !found {
		t.Error("restartEnv did not preserve an explicit OVERVIEW_DATA_DIR")
	}
}

func TestDesktopURL(t *testing.T) {
	got, err := desktopURL("127.0.0.1:5230")
	if err != nil {
		t.Fatalf("desktopURL: %v", err)
	}
	if got != "http://127.0.0.1:5230/?desktop=1" {
		t.Errorf("desktopURL = %q", got)
	}

	if _, err := desktopURL("not-an-address"); err == nil {
		t.Error("desktopURL should reject an address without a port")
	}
}

func TestIsDeepLink(t *testing.T) {
	cases := map[string]bool{
		"overview://note/a.md":      true,
		"OVERVIEW://OPEN?path=a.md": true,
		"https://example.com":       false,
		"C:\\notes\\a.md":           false,
		"/tmp/a.md":                 false,
		"":                          false,
	}
	for in, want := range cases {
		if got := isDeepLink(in); got != want {
			t.Errorf("isDeepLink(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDeepLink(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"overview://note/notes/a.md", "notes/a.md", true},
		{"overview://note/?path=notes/b.md", "notes/b.md", true},
		{"overview://open?path=notes/c.md", "notes/c.md", true},
		{"overview://note/", "", false},
		{"overview://unknown/notes/a.md", "", false},
		{"https://example.com/note/a.md", "", false},
		{"not a url", "", false},
	}
	for _, tc := range cases {
		got, ok := parseDeepLink(tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseDeepLink(%q) = (%q, %v), want (%q, %v)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

func TestEncodeRelPath(t *testing.T) {
	got := encodeRelPath("notes/a b/c#d.md")
	if got != "notes/a%20b/c%23d.md" {
		t.Errorf("encodeRelPath = %q, want notes/a%%20b/c%%23d.md", got)
	}
}

func TestHandleOpenQueuesUntilReady(t *testing.T) {
	d := &desktop{}
	d.handleOpen("overview://note/notes/a.md")
	d.handleOpen("   ")
	d.handleOpen(`C:\tmp\b.md`)

	d.mu.Lock()
	pending := append([]string(nil), d.pending...)
	d.mu.Unlock()

	if len(pending) != 2 {
		t.Fatalf("pending = %v, want two queued targets", pending)
	}
	if pending[0] != "overview://note/notes/a.md" || pending[1] != `C:\tmp\b.md` {
		t.Errorf("pending = %v, want [deep link, file path]", pending)
	}
}

func TestUnknownDeepLinkFeedbackIsNonBlocking(t *testing.T) {
	var calls int
	d := &desktop{
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		problemReporter: func(string, string) { calls++ },
	}

	done := make(chan struct{})
	go func() {
		d.openDeepLink("overview://unknown/path")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("openDeepLink blocked reporting an unknown link")
	}
	if calls != 1 {
		t.Errorf("problemReporter calls = %d, want 1", calls)
	}
}

func TestDesktopLogFileDefaultsToDataDir(t *testing.T) {
	dir := filepath.Join("vault")
	got := desktopLogFile(config.Config{DataDir: dir})
	want := filepath.Join(dir, "logs", "desktop.log")
	if got != want {
		t.Errorf("desktopLogFile = %q, want %q", got, want)
	}
}

func TestDesktopLogFileHonoursExplicitPath(t *testing.T) {
	got := desktopLogFile(config.Config{DataDir: "vault", LogFile: "custom.log"})
	if got != "custom.log" {
		t.Errorf("desktopLogFile = %q, want custom.log", got)
	}
}
