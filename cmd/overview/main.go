package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/overview-app/overview/internal/config"
	"github.com/overview-app/overview/internal/index"
	"github.com/overview-app/overview/internal/mcp"
	"github.com/overview-app/overview/internal/server"
	"github.com/overview-app/overview/internal/service"
	"github.com/overview-app/overview/internal/store"
	"github.com/overview-app/overview/internal/webui"
)

// version is overridden at build time with -ldflags="-X main.version=...".
var version = "0.2.0-dev"

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

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

	svc := service.New(st, idx, st)
	auth := service.NewAuth(idx, cfg.AuthMode)
	_ = idx.DeleteExpiredSessions(context.Background(), time.Now().UTC())

	var mcpHandler http.Handler
	{
		var verify mcp.TokenVerifier
		switch {
		case cfg.MCPToken != "":
			token := cfg.MCPToken
			verify = func(_ context.Context, provided string) error {
				if provided != token {
					return errors.New("invalid token")
				}
				return nil
			}
		case auth.Required():
			verify = func(ctx context.Context, provided string) error {
				_, err := auth.Authenticate(ctx, provided)
				return err
			}
		}
		mcpHandler = mcp.New(svc, verify)
	}

	dav := server.NewDAV(st.NotesDir(), auth, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := svc.Reindex(ctx); err != nil {
			logger.Warn("reindex after WebDAV write failed", "error", err)
		}
	}, logger)
	srv := server.New(svc, server.Options{
		MaxUploadBytes: cfg.MaxUploadMB << 20,
		Static:         staticFS,
		Version:        version,
		Logger:         logger,
		Auth:           auth,
		DAV:            dav,
		MCP:            mcpHandler,
	})
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Warn("shutdown", "error", err)
	}
	logger.Info("stopped")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
