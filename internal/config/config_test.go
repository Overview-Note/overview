package config_test

import (
	"path/filepath"
	"testing"

	"github.com/Overview-Note/overview/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	// Ensure a clean slate for the variables we assert on.
	for _, k := range []string{
		"OVERVIEW_ADDR", "OVERVIEW_AUTH", "OVERVIEW_HISTORY_KEEP", "OVERVIEW_MAX_UPLOAD_MB",
		"OVERVIEW_LOG_FORMAT", "OVERVIEW_LOG_FILE", "OVERVIEW_LOG_MAX_MB", "OVERVIEW_LOG_BACKUPS",
		"OVERVIEW_RENDER", "OVERVIEW_SITE_TITLE", "OVERVIEW_DATA_DIR",
		"OVERVIEW_DESKTOP", "OVERVIEW_ENABLE_MCP", "OVERVIEW_ENABLE_DAV",
	} {
		t.Setenv(k, "")
	}
	cfg := config.Load()
	if cfg.Addr != ":5230" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.AuthMode != "multi" {
		t.Errorf("AuthMode = %q", cfg.AuthMode)
	}
	if cfg.HistoryKeep != 50 {
		t.Errorf("HistoryKeep = %d", cfg.HistoryKeep)
	}
	if cfg.MaxUploadMB != 32 {
		t.Errorf("MaxUploadMB = %d", cfg.MaxUploadMB)
	}
	if cfg.LogFormat != "json" || cfg.LogMaxMB != 10 || cfg.LogBackups != 3 {
		t.Errorf("log defaults = %q/%d/%d", cfg.LogFormat, cfg.LogMaxMB, cfg.LogBackups)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("OVERVIEW_ADDR", "127.0.0.1:9999")
	t.Setenv("OVERVIEW_AUTH", "none")
	t.Setenv("OVERVIEW_HISTORY_KEEP", "5")
	t.Setenv("OVERVIEW_LOG_FORMAT", "text")
	t.Setenv("OVERVIEW_LOG_FILE", "/tmp/overview.log")
	t.Setenv("OVERVIEW_LOG_MAX_MB", "20")
	t.Setenv("OVERVIEW_LOG_BACKUPS", "7")
	t.Setenv("OVERVIEW_RENDER", "true")
	t.Setenv("OVERVIEW_SITE_TITLE", "My Docs")
	t.Setenv("OVERVIEW_DATA_DIR", "/tmp/ov")

	cfg := config.Load()
	if cfg.Addr != "127.0.0.1:9999" || cfg.AuthMode != "none" || cfg.HistoryKeep != 5 {
		t.Fatalf("unexpected: %+v", cfg)
	}
	if cfg.LogFormat != "text" || cfg.LogFile != "/tmp/overview.log" || cfg.LogMaxMB != 20 || cfg.LogBackups != 7 {
		t.Fatalf("log overrides not applied: %+v", cfg)
	}
	if !cfg.Render || cfg.SiteTitle != "My Docs" {
		t.Fatalf("render/site title: %+v", cfg)
	}
	if cfg.NotesDir != filepath.Join("/tmp/ov", "notes") {
		t.Errorf("NotesDir = %q", cfg.NotesDir)
	}
}

func TestLoadDesktopDefaults(t *testing.T) {
	for _, k := range []string{
		"OVERVIEW_ADDR", "OVERVIEW_AUTH", "OVERVIEW_DATA_DIR",
		"OVERVIEW_ENABLE_MCP", "OVERVIEW_ENABLE_DAV", "OVERVIEW_RENDER",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("OVERVIEW_DESKTOP", "true")

	cfg := config.Load()
	if !cfg.Desktop {
		t.Fatal("Desktop = false, want true")
	}
	if cfg.Addr != "127.0.0.1:5230" {
		t.Errorf("Addr = %q, want 127.0.0.1:5230", cfg.Addr)
	}
	if cfg.AuthMode != "none" {
		t.Errorf("AuthMode = %q, want none", cfg.AuthMode)
	}
	if cfg.DataDir != config.DefaultDataDir() {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, config.DefaultDataDir())
	}
	if cfg.EnableMCP || cfg.EnableDAV {
		t.Errorf("MCP/DAV should default off on desktop: %v/%v", cfg.EnableMCP, cfg.EnableDAV)
	}
}

func TestLoadDesktopEnvOverrides(t *testing.T) {
	t.Setenv("OVERVIEW_DESKTOP", "true")
	t.Setenv("OVERVIEW_ADDR", "0.0.0.0:9999")
	t.Setenv("OVERVIEW_AUTH", "multi")
	t.Setenv("OVERVIEW_ENABLE_MCP", "true")
	t.Setenv("OVERVIEW_ENABLE_DAV", "true")
	t.Setenv("OVERVIEW_DATA_DIR", "/tmp/desktop-ov")

	cfg := config.Load()
	if cfg.Addr != "0.0.0.0:9999" || cfg.AuthMode != "multi" {
		t.Fatalf("env overrides not applied: %+v", cfg)
	}
	if !cfg.EnableMCP || !cfg.EnableDAV {
		t.Errorf("env should enable MCP/DAV: %v/%v", cfg.EnableMCP, cfg.EnableDAV)
	}
	if cfg.DataDir != "/tmp/desktop-ov" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
}

func TestDefaultDataDirNonEmpty(t *testing.T) {
	if got := config.DefaultDataDir(); got == "" {
		t.Error("DefaultDataDir returned empty string")
	}
}

func TestLoadInvalidFallsBack(t *testing.T) {
	t.Setenv("OVERVIEW_MAX_UPLOAD_MB", "not-a-number")
	t.Setenv("OVERVIEW_HISTORY_KEEP", "-3")
	cfg := config.Load()
	if cfg.MaxUploadMB != 32 {
		t.Errorf("MaxUploadMB = %d, want 32", cfg.MaxUploadMB)
	}
	if cfg.HistoryKeep != 50 {
		t.Errorf("HistoryKeep = %d, want 50", cfg.HistoryKeep)
	}
}
