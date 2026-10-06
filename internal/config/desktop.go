package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DesktopConfig is the persisted configuration of the desktop shell. It lives
// in the per-user configuration directory rather than inside the vault, so the
// shell can remember where the vault is before one exists and without a
// chicken-and-egg dependency on the data directory.
type DesktopConfig struct {
	DataDir string `json:"dataDir"`
}

const desktopConfigFile = "desktop.json"

// DesktopConfigPath returns the absolute path of the desktop configuration
// file: <UserConfigDir>/Overview/desktop.json.
func DesktopConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "Overview", desktopConfigFile), nil
}

// LoadDesktopConfig reads the desktop configuration. A missing file is not an
// error and yields a zero DesktopConfig, so callers can treat "no config" and
// "empty config" alike.
func LoadDesktopConfig() (DesktopConfig, error) {
	path, err := DesktopConfigPath()
	if err != nil {
		return DesktopConfig{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DesktopConfig{}, nil
	}
	if err != nil {
		return DesktopConfig{}, fmt.Errorf("read desktop config: %w", err)
	}
	// Tolerate a UTF-8 BOM, which Windows editors may prepend.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var cfg DesktopConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DesktopConfig{}, fmt.Errorf("parse desktop config: %w", err)
	}
	return cfg, nil
}

// SaveDesktopConfig writes the desktop configuration, creating its directory
// when needed. The write is atomic so a crash cannot leave a truncated file.
func SaveDesktopConfig(cfg DesktopConfig) error {
	path, err := DesktopConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write desktop config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace desktop config: %w", err)
	}
	return nil
}

// HasVault reports whether dir looks like an existing Overview vault: a
// directory holding either a notes/ folder or an overview.db index. It is used
// to validate a directory the user wants to open.
func HasVault(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "notes")); err == nil && info.IsDir() {
		return true
	}
	if info, err := os.Stat(filepath.Join(dir, "overview.db")); err == nil && !info.IsDir() {
		return true
	}
	return false
}

// PrepareDataDir validates that dir can serve as an Overview vault, creates the
// directory structure when it does not exist yet and verifies it is writable.
// It returns the absolute, cleaned path.
//
// A missing or empty directory is treated as a new vault. An existing
// non-empty directory must already look like a vault (see HasVault); anything
// else is rejected so a stray folder is never silently turned into a vault.
func PrepareDataDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", errors.New("data directory is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	switch {
	case err == nil && !info.IsDir():
		return "", fmt.Errorf("not a directory: %s", abs)
	case err == nil && !isDirEmpty(abs) && !HasVault(abs):
		return "", fmt.Errorf("not an Overview vault (missing notes/ or overview.db): %s", abs)
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("read data directory: %w", err)
	}
	cfg := Config{}.WithDataDir(abs)
	if err := cfg.EnsureDirs(); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	if err := checkWritable(abs); err != nil {
		return "", err
	}
	return abs, nil
}

// isDirEmpty reports whether dir contains no entries.
func isDirEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return errors.Is(err, io.EOF)
}

// checkWritable verifies the process can create files in dir.
func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".overview-write-*")
	if err != nil {
		return fmt.Errorf("data directory is not writable: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("data directory is not writable: %w", err)
	}
	return nil
}
