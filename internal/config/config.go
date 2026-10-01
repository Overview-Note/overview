package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds runtime configuration resolved from environment variables.
type Config struct {
	Addr        string
	DataDir     string
	NotesDir    string
	AssetsDir   string
	DBPath      string
	HistoryDir  string
	TrashDir    string
	MaxUploadMB int64
	LogLevel    string
	AuthMode    string
	MCPToken    string
	AIBaseURL   string
	AIAPIKey    string
	AIModel     string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load builds a Config from environment variables with sensible defaults.
func Load() Config {
	dataDir := env("OVERVIEW_DATA_DIR", "./data")
	maxUpload, err := strconv.ParseInt(env("OVERVIEW_MAX_UPLOAD_MB", "32"), 10, 64)
	if err != nil || maxUpload <= 0 {
		maxUpload = 32
	}
	return Config{
		Addr:        env("OVERVIEW_ADDR", ":5230"),
		DataDir:     dataDir,
		NotesDir:    filepath.Join(dataDir, "notes"),
		AssetsDir:   filepath.Join(dataDir, "assets"),
		DBPath:      env("OVERVIEW_DB", filepath.Join(dataDir, "overview.db")),
		HistoryDir:  filepath.Join(dataDir, ".history"),
		TrashDir:    filepath.Join(dataDir, ".trash"),
		MaxUploadMB: maxUpload,
		LogLevel:    env("OVERVIEW_LOG_LEVEL", "info"),
		AuthMode:    env("OVERVIEW_AUTH", "multi"),
		MCPToken:    env("OVERVIEW_MCP_TOKEN", ""),
		AIBaseURL:   env("OVERVIEW_AI_BASE_URL", ""),
		AIAPIKey:    env("OVERVIEW_AI_API_KEY", ""),
		AIModel:     env("OVERVIEW_AI_MODEL", "gpt-4o-mini"),
	}
}

// EnsureDirs creates the directories Overview depends on if they do not exist.
func (c Config) EnsureDirs() error {
	for _, dir := range []string{c.DataDir, c.NotesDir, c.AssetsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
