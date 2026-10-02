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
	HistoryKeep int
	LogLevel    string
	AuthMode    string
	MCPToken    string
	AIBaseURL   string
	AIAPIKey    string
	AIModel     string
	S3Endpoint  string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool
	S3PublicURL string
	SiteTitle   string
	Render      bool
}

// S3Enabled reports whether an S3-compatible asset backend is configured.
func (c Config) S3Enabled() bool { return c.S3Bucket != "" }

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
	historyKeep, err := strconv.Atoi(env("OVERVIEW_HISTORY_KEEP", "50"))
	if err != nil || historyKeep < 0 {
		historyKeep = 50
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
		HistoryKeep: historyKeep,
		LogLevel:    env("OVERVIEW_LOG_LEVEL", "info"),
		AuthMode:    env("OVERVIEW_AUTH", "multi"),
		MCPToken:    env("OVERVIEW_MCP_TOKEN", ""),
		AIBaseURL:   env("OVERVIEW_AI_BASE_URL", ""),
		AIAPIKey:    env("OVERVIEW_AI_API_KEY", ""),
		AIModel:     env("OVERVIEW_AI_MODEL", "gpt-4o-mini"),
		S3Endpoint:  env("OVERVIEW_S3_ENDPOINT", ""),
		S3Region:    env("OVERVIEW_S3_REGION", "us-east-1"),
		S3AccessKey: env("OVERVIEW_S3_ACCESS_KEY", ""),
		S3SecretKey: env("OVERVIEW_S3_SECRET_KEY", ""),
		S3Bucket:    env("OVERVIEW_S3_BUCKET", ""),
		S3UseSSL:    env("OVERVIEW_S3_USE_SSL", "true") == "true",
		S3PublicURL: env("OVERVIEW_S3_PUBLIC_URL", ""),
		SiteTitle:   env("OVERVIEW_SITE_TITLE", "Overview"),
		Render:      env("OVERVIEW_RENDER", "") == "true",
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
