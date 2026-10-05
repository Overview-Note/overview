// Package app wires the Overview service, index and HTTP server into a
// reusable, lifecycle-managed application. It is shared by the CLI entry point
// and, later, the desktop shell.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Overview-Note/overview/internal/agent"
	"github.com/Overview-Note/overview/internal/config"
	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/mcp"
	"github.com/Overview-Note/overview/internal/s3store"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
	"github.com/Overview-Note/overview/internal/trash"
	"github.com/Overview-Note/overview/internal/watcher"
	"github.com/Overview-Note/overview/internal/webui"
)

// Version is reported by the API and the MCP server. The entry point overrides
// it with the linker-injected build version.
var Version = "dev"

const reindexTimeout = 5 * time.Minute

// App owns the long-lived resources of an Overview server process.
type App struct {
	cfg    config.Config
	logger *slog.Logger
	opts   Options

	svc    *service.Service
	store  *store.Store
	index  *index.Index
	server *server.Server
	http   *http.Server

	listener net.Listener
	lock     *InstanceLock
	portPath string

	watchCancel context.CancelFunc
	watcherDone chan struct{}

	reindexCancel context.CancelFunc
	reindexDone   chan struct{}

	stopOnce sync.Once
	stopErr  error
}

// Options carries optional dependencies injected by a host process.
type Options struct {
	// DesktopHooks exposes desktop-shell capabilities to the HTTP API. It is
	// nil for a headless server.
	DesktopHooks server.DesktopHooks
	// UpdateCheck overrides the GitHub release lookup used by the desktop
	// update endpoint. It is nil in production, where update.CheckLatest is
	// used.
	UpdateCheck server.UpdateChecker
}

// New assembles the application: it prepares the data directories, takes the
// single-instance lock and constructs the service, index and HTTP server. It
// does not bind a port or start indexing; call Start for that.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	return NewWithOptions(cfg, logger, Options{})
}

// NewWithOptions is New with host-provided dependencies.
func NewWithOptions(cfg config.Config, logger *slog.Logger, opts Options) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := cfg.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("prepare data dirs: %w", err)
	}
	lock, err := AcquireInstanceLock(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	a := &App{cfg: cfg, logger: logger, opts: opts, lock: lock}
	if err := a.build(); err != nil {
		_ = a.releaseResources()
		return nil, err
	}
	return a, nil
}

// Handler returns the composed HTTP handler.
func (a *App) Handler() http.Handler { return a.server.Handler() }

func (a *App) build() error {
	cfg := a.cfg

	st := store.New(cfg.NotesDir, cfg.AssetsDir)
	a.store = st

	idx, err := index.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	a.index = idx

	var staticFS fs.FS
	if sub, err := webui.FS(); err == nil {
		staticFS = sub
	}

	hist := history.New(cfg.HistoryDir, cfg.HistoryKeep)
	tr := trash.New(cfg.TrashDir)

	assets := service.NewSwitchableAssetStore(st)
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
			return fmt.Errorf("init s3 asset store: %w", err)
		}
		assets.Set(s3)
		a.logger.Info("using s3 asset backend", "bucket", cfg.S3Bucket)
	}

	svc := service.New(st, idx, assets, hist, tr)
	a.svc = svc

	auth := service.NewAuth(idx, cfg.AuthMode)
	tokens := service.NewTokenService(idx)
	_ = idx.DeleteExpiredSessions(context.Background(), time.Now().UTC())
	_ = idx.DeleteExpiredUserTokens(context.Background(), time.Now().UTC())

	verify := tokenVerifier(cfg.MCPToken, auth, tokens)

	var mcpHandler http.Handler
	if cfg.EnableMCP {
		mcpHandler = mcp.New(svc, verify)
	}
	mcp.ServerVersion = Version

	var dav http.Handler
	if cfg.EnableDAV {
		dav = server.NewDAV(st.NotesDir(), auth, func(paths []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			for _, p := range paths {
				if err := svc.ReindexPath(ctx, p); err != nil {
					a.logger.Warn("reindex after WebDAV write failed", "path", p, "error", err)
				}
			}
		}, a.logger)
		if !auth.Required() {
			// Anonymous WebDAV is never acceptable; require a token instead.
			dav = server.RequireToken(dav, server.TokenVerifier(verify))
		}
	}

	aiSvc := service.NewAI(idx, service.AIConfig{
		BaseURL: cfg.AIBaseURL,
		APIKey:  cfg.AIAPIKey,
		Model:   cfg.AIModel,
	})
	aiSvc.Load(context.Background())
	agt := agent.New(svc, aiSvc, idx)
	mailSvc := service.NewMail(idx, service.MailConfig{
		Host:     cfg.MailHost,
		Port:     cfg.MailPort,
		Username: cfg.MailUsername,
		Password: cfg.MailPassword,
		From:     cfg.MailFrom,
		StartTLS: cfg.MailStartTLS,
	})
	mailSvc.Load(context.Background())
	auth.SetMailer(mailSvc)
	siteSvc := service.NewSite(idx)
	siteSvc.Load(context.Background())

	storageSvc := service.NewStorage(idx, st, assets)
	storageSvc.SetDefaults(service.StorageConfig{
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Bucket:    cfg.S3Bucket,
		UseSSL:    cfg.S3UseSSL,
		PublicURL: cfg.S3PublicURL,
	})
	if err := storageSvc.Load(context.Background()); err != nil {
		a.logger.Warn("load storage settings", "error", err)
	}

	a.server = server.New(svc, server.Options{
		MaxUploadBytes: cfg.MaxUploadMB << 20,
		Static:         staticFS,
		Version:        Version,
		Logger:         a.logger,
		Auth:           auth,
		Tokens:         tokens,
		DAV:            dav,
		MCP:            mcpHandler,
		AI:             aiSvc,
		Agent:          agt,
		Mail:           mailSvc,
		Site:           siteSvc,
		Storage:        storageSvc,
		BaseURL:        cfg.BaseURL,
		Render:         cfg.Render,
		SiteTitle:      cfg.SiteTitle,
		LocalOnly:      cfg.Desktop,
		DesktopHooks:   a.opts.DesktopHooks,
		UpdateCheck:    a.opts.UpdateCheck,
	})
	if cfg.Render {
		a.logger.Info("running in render mode (public documentation site, read-only)")
	}
	if auth.Required() {
		if needs, err := auth.NeedsSetup(context.Background()); err == nil && needs {
			a.logger.Info("authentication enabled; first-run setup required at /setup")
		}
	}
	return nil
}

// tokenVerifier builds a verifier shared by the MCP and WebDAV endpoints. It
// never returns a nil verifier, so token checks are always enforced.
func tokenVerifier(mcpToken string, auth *service.AuthService, tokens *service.TokenService) func(context.Context, string) error {
	return func(ctx context.Context, provided string) error {
		if mcpToken != "" && provided == mcpToken {
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
}

// Start binds the listener and begins serving. It returns the actual address
// the server is listening on. The initial reindex runs in the background, so
// the health endpoint is usable as a readiness probe immediately.
func (a *App) Start(ctx context.Context) (string, error) {
	ln, err := a.listen()
	if err != nil {
		return "", err
	}
	a.listener = ln
	addr := ln.Addr().String()

	if path, err := writePortFile(a.cfg.DataDir, addr); err != nil {
		a.logger.Warn("write port file", "error", err)
	} else {
		a.portPath = path
	}

	a.http = &http.Server{
		Handler:           a.server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	a.logger.Info("Overview listening", "addr", addr, "data", a.cfg.DataDir, "version", Version)

	a.watchCancel, a.watcherDone = a.startWatcher()

	reindexCtx, cancelReindex := context.WithTimeout(ctx, reindexTimeout)
	a.reindexCancel = cancelReindex
	a.reindexDone = make(chan struct{})
	go func() {
		defer close(a.reindexDone)
		defer cancelReindex()
		a.reindex(reindexCtx)
	}()

	go func() {
		if err := a.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("server error", "error", err)
		}
	}()

	return addr, nil
}

// listen binds the configured address. In desktop mode a busy fixed port falls
// back to a random loopback port so the app can always start; self-hosted
// servers keep failing loudly.
func (a *App) listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", a.cfg.Addr)
	if err == nil {
		return ln, nil
	}
	if !a.cfg.Desktop {
		return nil, fmt.Errorf("listen on %s: %w", a.cfg.Addr, err)
	}
	a.logger.Warn("preferred port unavailable, falling back to a random loopback port",
		"addr", a.cfg.Addr, "error", err)
	return net.Listen("tcp", "127.0.0.1:0")
}

func (a *App) startWatcher() (context.CancelFunc, chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := watcher.New(a.cfg.NotesDir, a.svc, a.logger).Run(ctx); err != nil {
			a.logger.Warn("file watcher stopped", "error", err)
		}
	}()
	return cancel, done
}

func (a *App) reindex(ctx context.Context) {
	start := time.Now()
	if err := a.svc.Reindex(ctx); err != nil {
		a.logger.Warn("initial reindex failed", "error", err)
		return
	}
	a.logger.Info("index ready", "duration_ms", time.Since(start).Milliseconds())
}

// Stop gracefully shuts the server down, stops the watcher and releases the
// index and single-instance lock. It is safe to call more than once.
func (a *App) Stop(ctx context.Context) error {
	a.stopOnce.Do(func() {
		// Stop background work and wait for it before closing the index.
		if a.reindexCancel != nil {
			a.reindexCancel()
		}
		if a.reindexDone != nil {
			select {
			case <-a.reindexDone:
			case <-ctx.Done():
			}
		}
		if a.watchCancel != nil {
			a.watchCancel()
		}
		if a.watcherDone != nil {
			select {
			case <-a.watcherDone:
			case <-ctx.Done():
			}
		}
		if a.http != nil {
			if err := a.http.Shutdown(ctx); err != nil {
				a.stopErr = err
			}
		}
		if err := a.releaseResources(); err != nil && a.stopErr == nil {
			a.stopErr = err
		}
		a.logger.Info("stopped")
	})
	return a.stopErr
}

func (a *App) releaseResources() error {
	var firstErr error
	if a.index != nil {
		if err := a.index.Close(); err != nil {
			firstErr = err
		}
		a.index = nil
	}
	if a.lock != nil {
		if err := a.lock.Release(); err != nil && firstErr == nil {
			firstErr = err
		}
		a.lock = nil
	}
	if err := removePortFile(a.portPath); err != nil && firstErr == nil {
		firstErr = err
	}
	a.portPath = ""
	return firstErr
}

// NotesDir returns the absolute directory holding the vault's Markdown notes.
func (a *App) NotesDir() string { return a.cfg.NotesDir }

// ResolveOrImport maps an external file to a vault-relative, slash-separated
// path. A file already inside the notes directory is returned unchanged; any
// other Markdown file is copied into the vault root, indexed and returned. The
// original file is never modified or removed.
func (a *App) ResolveOrImport(ctx context.Context, abs string) (string, error) {
	full, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", abs)
	}

	notesAbs, err := filepath.Abs(a.cfg.NotesDir)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(notesAbs, full); err == nil && rel != "." &&
		rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return filepath.ToSlash(rel), nil
	}
	return a.importFile(ctx, full)
}

// importFile copies a local Markdown file into the vault root under a name that
// does not collide with an existing note, then indexes it.
func (a *App) importFile(ctx context.Context, src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	base := filepath.Base(src)
	if !strings.EqualFold(filepath.Ext(base), ".md") {
		base += ".md"
	}
	rel := a.uniqueName(base)
	if err := a.store.WriteRaw(ctx, rel, data); err != nil {
		return "", fmt.Errorf("write note: %w", err)
	}
	if err := a.svc.ReindexPath(ctx, rel); err != nil {
		return "", fmt.Errorf("index note: %w", err)
	}
	return rel, nil
}

// uniqueName returns a vault-relative file name derived from base, appending a
// numeric suffix when a note with that name already exists.
func (a *App) uniqueName(base string) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	name := base
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(a.cfg.NotesDir, filepath.FromSlash(name))); os.IsNotExist(err) {
			return filepath.ToSlash(name)
		}
		if i > 10000 {
			return filepath.ToSlash(name)
		}
		name = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
}
