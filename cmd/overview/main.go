package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Overview-Note/overview/internal/app"
	"github.com/Overview-Note/overview/internal/cli"
	"github.com/Overview-Note/overview/internal/config"
	"github.com/Overview-Note/overview/internal/logging"
)

// version is overridden at build time with -ldflags="-X main.version=...".
var version = "0.16.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			serve(os.Args[2:])
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
	serve(nil)
}

// serve runs the HTTP server until the process receives a termination signal.
func serve(args []string) {
	cfg := config.Load()
	cfg = applyServeFlags(cfg, args)
	lg := setupLogger(cfg)
	defer lg.Close()
	logger := lg.Logger

	app.Version = version
	a, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("start Overview", "error", err)
		os.Exit(1)
	}

	if _, err := a.Start(context.Background()); err != nil {
		logger.Error("start Overview", "error", err)
		_ = a.Stop(context.Background())
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Stop(shutdownCtx); err != nil {
		logger.Warn("shutdown", "error", err)
	}
}

// applyServeFlags honours the global CLI flags that apply to the server:
// -C/--data-dir selects the vault directory.
func applyServeFlags(cfg config.Config, args []string) config.Config {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-C" || a == "--data-dir":
			if i+1 < len(args) {
				cfg = cfg.WithDataDir(args[i+1])
				i++
			}
		case strings.HasPrefix(a, "--data-dir="):
			cfg = cfg.WithDataDir(strings.TrimPrefix(a, "--data-dir="))
		}
	}
	return cfg
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
