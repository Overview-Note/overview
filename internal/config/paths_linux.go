//go:build linux

package config

import (
	"os"
	"path/filepath"
)

// DefaultDataDir is the per-user vault location for the desktop build on
// Linux: ~/Documents/Overview when Documents exists, otherwise the XDG data
// directory and finally the user configuration directory.
func DefaultDataDir() string {
	if dir, ok := documentsDataDir(); ok {
		return dir
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "overview")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "share", "overview")
	}
	return configDataDir()
}
