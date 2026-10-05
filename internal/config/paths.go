package config

import (
	"os"
	"path/filepath"
)

// documentsDataDir returns <home>/Documents/Overview when the Documents
// directory exists, and false otherwise.
func documentsDataDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	docs := filepath.Join(home, "Documents")
	if info, err := os.Stat(docs); err != nil || !info.IsDir() {
		return "", false
	}
	return filepath.Join(docs, "Overview"), true
}

// configDataDir is the final fallback used when no per-user Documents
// directory is available.
func configDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "Overview")
	}
	return "Overview"
}
