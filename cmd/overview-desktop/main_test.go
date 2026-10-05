//go:build windows || darwin || desktop

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
