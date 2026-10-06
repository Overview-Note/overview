package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Overview-Note/overview/internal/config"
)

// isolateConfigDir points os.UserConfigDir at a fresh temporary directory for
// the duration of the test.
func isolateConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)         // windows
	t.Setenv("XDG_CONFIG_HOME", dir) // linux
	t.Setenv("HOME", dir)            // darwin and linux home fallback
	t.Setenv("USERPROFILE", dir)     // windows home fallback
}

func TestDesktopConfigRoundTrip(t *testing.T) {
	isolateConfigDir(t)

	if _, err := config.LoadDesktopConfig(); err != nil {
		t.Fatalf("Load missing config: %v", err)
	}
	path, err := config.DesktopConfigPath()
	if err != nil {
		t.Fatalf("DesktopConfigPath: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config file should not exist yet: %v", err)
	}

	want := config.DesktopConfig{DataDir: "/tmp/vault"}
	if err := config.SaveDesktopConfig(want); err != nil {
		t.Fatalf("SaveDesktopConfig: %v", err)
	}
	got, err := config.LoadDesktopConfig()
	if err != nil {
		t.Fatalf("LoadDesktopConfig: %v", err)
	}
	if got.DataDir != want.DataDir {
		t.Errorf("DataDir = %q, want %q", got.DataDir, want.DataDir)
	}
}

func TestLoadDesktopConfigToleratesBOM(t *testing.T) {
	isolateConfigDir(t)

	path, err := config.DesktopConfigPath()
	if err != nil {
		t.Fatalf("DesktopConfigPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"dataDir":"/bom"}`)...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := config.LoadDesktopConfig()
	if err != nil {
		t.Fatalf("LoadDesktopConfig: %v", err)
	}
	if got.DataDir != "/bom" {
		t.Errorf("DataDir = %q, want /bom", got.DataDir)
	}
}

func TestPrepareDataDirCreatesNewVault(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-vault")

	got, err := config.PrepareDataDir(dir)
	if err != nil {
		t.Fatalf("PrepareDataDir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("result %q is not absolute", got)
	}
	for _, sub := range []string{"notes", "assets"} {
		if info, err := os.Stat(filepath.Join(dir, sub)); err != nil || !info.IsDir() {
			t.Errorf("%s not created: %v", sub, err)
		}
	}
}

func TestPrepareDataDirAcceptsExistingVault(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if _, err := config.PrepareDataDir(dir); err != nil {
		t.Fatalf("PrepareDataDir existing vault: %v", err)
	}
	if !config.HasVault(dir) {
		t.Error("HasVault = false, want true")
	}
}

func TestPrepareDataDirAcceptsOverviewDB(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "overview.db"), []byte("x"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, err := config.PrepareDataDir(dir); err != nil {
		t.Fatalf("PrepareDataDir with overview.db: %v", err)
	}
}

func TestPrepareDataDirRejectsNonVault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, err := config.PrepareDataDir(dir); err == nil {
		t.Fatal("PrepareDataDir accepted a non-empty non-vault directory")
	}
}

func TestPrepareDataDirRejectsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, err := config.PrepareDataDir(file); err == nil {
		t.Fatal("PrepareDataDir accepted a file path")
	}
}

func TestPrepareDataDirRejectsEmpty(t *testing.T) {
	if _, err := config.PrepareDataDir("   "); err == nil {
		t.Fatal("PrepareDataDir accepted an empty path")
	}
}
