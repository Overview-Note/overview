package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Overview-Note/overview/internal/cli"
	"github.com/Overview-Note/overview/internal/config"
	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/logging"
	"github.com/Overview-Note/overview/internal/mcp"
	"github.com/Overview-Note/overview/internal/s3store"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
	"github.com/Overview-Note/overview/internal/trash"
	"github.com/Overview-Note/overview/internal/watcher"
	"github.com/Overview-Note/overview/internal/webui"
)

// version is overridden at build time with -ldflags="-X main.version=...".
var version = "0.11.1"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			serve()
			return
		case "version", "--version", "-v":
			fmt.Println("overview", version)
			return
		}
		os.Exit(cli.Run(os.Args[1:], cli.Env{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}))
	}
	serve()
}

// serve runs the HTTP server until the process receives a termination signal.
func serve() {
	cfg := config.Load()
	lg := setupLogger(cfg)
	defer lg.Close()
	logger := lg.Logger

	if err := cfg.EnsureDirs(); err != nil {
		logger.Error("prepare data dirs", "error", err)
		os.Exit(1)
	}

	st := store.New(cfg.NotesDir, cfg.AssetsDir)

	idx, err := index.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open index", "error", err)
		os.Exit(1)
	}
	defer idx.Close()

	var staticFS fs.FS
	if sub, err := webui.FS(); err == nil {
		staticFS = sub
	}

	hist := history.New(cfg.HistoryDir, cfg.HistoryKeep)
	tr := trash.New(cfg.TrashDir)

	var assets core.AssetStore = st
	if cfg.S3Enabled() {
		s3, err := s3store.New(context.Background(), s3store.Options{
			Endpoint:  cfg.S3Endpoint,
			Region:    cfg.S3Region,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			Bucket:    cfg.S3Bucket,
			UseSSL:    cfg.S3UseSSL,
			PublicURL: cfg.S3PublicURL,
		})
		if err != nil {
			logger.Error("init s3 asset store", "error", err)
			os.Exit(1)
		}
		assets = s3
		logger.Info("using s3 asset backend", "bucket", cfg.S3Bucket)
	}

	svc := service.New(st, idx, assets, hist, tr)
	auth := service.NewAuth(idx, cfg.AuthMode)
	tokens := service.NewTokenService(idx)
	_ = idx.DeleteExpiredSessions(context.Background(), time.Now().UTC())

	var mcpHandler http.Handler
	{
		var verify mcp.TokenVerifier
		switch {
		case cfg.MCPToken != "":
			token := cfg.MCPToken
			verify = func(ctx context.Context, provided string) error {
				if provided == token {
					return nil
				}
				if _, err := tokens.Verify(ctx, provided); err == nil {
					return nil
				}
				if _, err := auth.Authenticate(ctx, provided); err == nil {
					return nil
				}
				return errors.New("invalid token")
			}
		case auth.Required():
			verify = func(ctx context.Context, provided string) error {
				if _, err := tokens.Verify(ctx, provided); err == nil {
					return nil
				}
				_, err := auth.Authenticate(ctx, provided)
				return err
			}
		}
		mcpHandler = mcp.New(svc, verify)
	}
	mcp.ServerVersion = version

	dav := server.NewDAV(st.NotesDir(), auth, func(paths []string) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, p := range paths {
			if err := svc.ReindexPath(ctx, p); err != nil {
				logger.Warn("reindex after WebDAV write failed", "path", p, "error", err)
			}
		}
	}, logger)
	aiSvc := service.NewAI(idx, service.AIConfig{
		BaseURL: cfg.AIBaseURL,
		APIKey:  cfg.AIAPIKey,
		Model:   cfg.AIModel,
	})
	aiSvc.Load(context.Background())

	srv := server.New(svc, server.Options{
		MaxUploadBytes: cfg.MaxUploadMB << 20,
		Static:         staticFS,
		Version:        version,
		Logger:         logger,
		Auth:           auth,
		Tokens:         tokens,
		DAV:            dav,
		MCP:            mcpHandler,
		AI:             aiSvc,
		Render:         cfg.Render,
		SiteTitle:      cfg.SiteTitle,
	})
	if cfg.Render {
		logger.Info("running in render mode (public documentation site, read-only)")
	}
	if auth.Required() {
		if needs, err := auth.NeedsSetup(context.Background()); err == nil && needs {
			logger.Info("authentication enabled; first-run setup required at /setup")
		}
	}

	reindexCtx, cancelReindex := context.WithTimeout(context.Background(), 5*time.Minute)
	start := time.Now()
	if err := svc.Reindex(reindexCtx); err != nil {
		logger.Warn("initial reindex failed", "error", err)
	} else {
		logger.Info("index ready", "duration_ms", time.Since(start).Milliseconds())
	}
	cancelReindex()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("Overview listening", "addr", cfg.Addr, "data", cfg.DataDir, "version", version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()
	go func() {
		if err := watcher.New(st.NotesDir(), svc, logger).Run(watchCtx); err != nil {
			logger.Warn("file watcher stopped", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancelWatch()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Warn("shutdown", "error", err)
	}
	logger.Info("stopped")
}

// setupLogger builds the application logger (stdout + optional rotating file)
// and installs it as the slog default.
func setupLogger(cfg config.Config) *logging.Logger {
	lg, err := logging.New(logging.Options{
		Level:   cfg.LogLevel,
		Format:  cfg.LogFormat,
		File:    cfg.LogFile,
		MaxMB:   cfg.LogMaxMB,
		Backups: cfg.LogBackups,
	})
	if err != nil {
		// Fall back to stdout-only so logging never blocks startup.
		lg, _ = logging.New(logging.Options{Level: cfg.LogLevel, Format: cfg.LogFormat})
	}
	slog.SetDefault(lg.Logger)
	return lg
}
