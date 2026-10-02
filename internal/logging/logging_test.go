package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelInfo,
		"unknown": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q)=%v, want %v", in, got, want)
		}
	}
}

func TestLoggerWritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "overview.log")
	lg, err := New(Options{Level: "info", Format: "json", File: path})
	if err != nil {
		t.Fatal(err)
	}
	lg.Info("hello", "n", 1)
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"msg":"hello"`)) {
		t.Fatalf("log file missing message: %s", data)
	}
}

func TestRotator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	r, err := newRotator(path, 1, 2) // 1 MB max, 2 backups
	if err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte("x"), 600*1024)
	for i := 0; i < 3; i++ { // ~1.8 MB total -> at least one rotation
		if _, err := r.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file %s.1: %v", path, err)
	}
}

func TestTextFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.log")
	lg, err := New(Options{Level: "debug", Format: "text", File: path})
	if err != nil {
		t.Fatal(err)
	}
	lg.Debug("dbg")
	lg.Close()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "level=DEBUG") {
		t.Fatalf("expected text handler output, got: %s", data)
	}
}
