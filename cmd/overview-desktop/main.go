//go:build windows || darwin || desktop

// Command overview-desktop runs Overview inside a native desktop shell. It
// reuses the same local server as the headless build (internal/app) and hosts
// the web UI in a Wails v3 webview pointed at the loopback address. The shell
// only provides a window, a system tray and lifecycle handling; all
// application behaviour lives in the shared packages.
//
// The `desktop` build tag keeps the Wails dependency out of plain
// `go build ./...` on platforms that would need CGO and GTK headers (CI runs on
// Linux without those). Windows and macOS are built by default; Linux requires
// the tag.
//
// Requirements and build commands:
//
//	Windows (WebView2 runtime must be installed):
//	  go build ./cmd/overview-desktop
//	macOS (CGO, Xcode command line tools):
//	  go build -tags desktop ./cmd/overview-desktop
//	Linux (CGO + GTK4 + WebKitGTK 6.0, or GTK3 for Ubuntu 22.04/Debian 12):
//	  go build -tags desktop ./cmd/overview-desktop
//	  go build -tags "desktop gtk3" ./cmd/overview-desktop
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Overview-Note/overview/internal/app"
	"github.com/Overview-Note/overview/internal/config"
	"github.com/Overview-Note/overview/internal/logging"
	overviewsync "github.com/Overview-Note/overview/internal/sync"
	"github.com/Overview-Note/overview/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed icon.png
var iconPNG []byte

// version is overridden at build time with -ldflags="-X main.version=...".
var version = "0.14.0"

// shutdownTimeout bounds how long the local server may take to drain.
const shutdownTimeout = 10 * time.Second

func main() {
	ensureDesktopEnv()
	waitForPredecessor()

	base := config.Load()
	flags := parseFlags(os.Args[1:], base)

	lg := setupLogger(flags.cfg)
	defer lg.Close()
	app.Version = version
	lg.Info("desktop shell starting", "version", version, "dataDir", flags.cfg.DataDir)

	d := &desktop{
		cfg:             flags.cfg,
		explicitDataDir: flags.explicitDataDir,
		logger:          lg.Logger,
	}

	// The notifications service is created up front so it can be registered
	// with the Wails application. Its startup runs on the Wails main loop; the
	// response callback is wired below, once the application exists.
	d.notifier = notifications.New()

	// The Wails application must be created before the shared server so that a
	// second launch is detected (and the first window raised) without touching
	// the vault. internal/app's DataDir lock is a separate concern: it keeps a
	// single server per data directory, while the Wails unique ID keeps a
	// single window per application.
	d.wails = application.New(application.Options{
		Name:        "Overview",
		Description: "Overview desktop shell",
		Icon:        iconPNG,
		Logger:      lg.Logger,
		// Windows matches this list against the launched argument to emit an
		// ApplicationOpenedWithFile event.
		FileAssociations: []string{".md"},
		Services: []application.Service{
			application.NewService(d.notifier),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.overview.app",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				d.showWindow()
				d.forwardArgs(data.Args)
			},
		},
		OnShutdown: d.stopServer,
	})

	d.notifier.OnNotificationResponse(func(result notifications.NotificationResult) {
		target, _ := result.Response.UserInfo["url"].(string)
		if target == "" {
			return
		}
		if err := d.wails.Browser.OpenURL(target); err != nil {
			d.logger.Warn("open update page", "error", err)
		}
	})

	// A file association or protocol launch delivers its payload through these
	// events on the first launch; later launches arrive via
	// OnSecondInstanceLaunch above.
	d.wails.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		if f := e.Context().Filename(); f != "" {
			d.handleOpen(f)
		}
	})
	d.wails.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		if u := e.Context().URL(); u != "" {
			d.handleOpen(u)
		}
	})

	// The data directory may require a native directory picker on first run, so
	// the service and window are created once the Wails main loop is running.
	d.wails.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		d.start()
	})

	if err := d.wails.Run(); err != nil {
		d.logger.Error("desktop shell exited", "error", err)
	}
	d.stopServer()
}

// desktop owns the desktop lifecycle: the shared application server, the
// webview window and the system tray.
type desktop struct {
	cfg             config.Config
	explicitDataDir bool
	logger          *slog.Logger
	wails           *application.App
	notifier        *notifications.NotificationService

	mu           sync.Mutex
	server       *app.App
	window       *application.WebviewWindow
	baseURL      string
	pending      []string
	lastNotified string

	// problemReporter overrides how open failures are surfaced. Production
	// leaves it nil so reportProblem raises a system notification; tests
	// inject a recorder to assert the error path returns without blocking.
	problemReporter func(title, message string)

	// onboard overrides the first-run directory flow. Production leaves it nil
	// so runOnboarding drives native dialogs; tests inject a stub.
	onboard func(def string) string
}

// start brings up the local server and, once it is listening, the window and
// tray. It runs on the Wails event goroutine started with the main loop.
func (d *desktop) start() {
	dataDir := d.resolveDataDir()
	_ = os.Setenv("OVERVIEW_DATA_DIR", dataDir)

	cfg := d.cfg.WithDataDir(dataDir)
	d.cfg = cfg

	server, err := app.NewWithOptions(cfg, d.logger, app.Options{
		DesktopHooks: &desktopHooks{desktop: d},
		SyncFactory:  desktopSyncFactory,
	})
	if err != nil {
		d.fatal("无法启动 Overview", err)
		return
	}
	d.mu.Lock()
	d.server = server
	d.mu.Unlock()

	addr, err := server.Start(context.Background())
	if err != nil {
		d.fatal("无法启动本地服务", err)
		return
	}

	base, err := desktopBaseURL(addr)
	if err != nil {
		d.fatal("无法确定本地地址", err)
		return
	}
	d.logger.Info("local server listening", "addr", addr, "url", base)
	d.mu.Lock()
	d.baseURL = base
	d.mu.Unlock()

	window := d.wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Overview",
		URL:            desktopURL(base, version),
		Width:          1280,
		Height:         820,
		MinWidth:       960,
		MinHeight:      600,
		EnableFileDrop: true,
	})
	d.mu.Lock()
	d.window = window
	d.mu.Unlock()

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		for _, path := range e.Context().DroppedFiles() {
			d.handleOpen(path)
		}
	})

	d.setupTray()
	d.showWindow()
	window.Center()

	d.drainPending()
	d.startUpdateChecks()
}

// resolveDataDir returns the vault directory. An explicit --data-dir or
// OVERVIEW_DATA_DIR always wins and skips onboarding. Otherwise the persisted
// desktop configuration is used; when there is none and the default directory
// is missing or empty, the first-run onboarding runs and its result is saved.
func (d *desktop) resolveDataDir() string {
	var persisted config.DesktopConfig
	if loaded, err := config.LoadDesktopConfig(); err != nil {
		d.logger.Warn("read desktop config", "error", err)
	} else {
		persisted = loaded
	}
	onboard := d.onboard
	if onboard == nil {
		onboard = d.runOnboarding
	}
	chosen, persist := chooseDataDir(
		d.explicitDataDir, d.cfg.DataDir, persisted, config.DefaultDataDir(), onboard,
	)
	if persist {
		d.persistDataDir(chosen)
	}
	return chosen
}

// chooseDataDir applies the startup precedence: explicit flag/env, then the
// persisted desktop config, then the default directory. When the default
// directory is missing or empty the onboarding callback decides, and its
// result (plus the default fallback) must be persisted. It reports whether the
// caller should write the resulting directory back to the desktop config.
func chooseDataDir(explicit bool, explicitDir string, persisted config.DesktopConfig, def string, onboarding func(string) string) (string, bool) {
	if explicit {
		return explicitDir, false
	}
	if persisted.DataDir != "" {
		return persisted.DataDir, false
	}
	if !needsBootstrap(def) {
		return def, true
	}
	chosen := onboarding(def)
	if chosen == "" {
		chosen = def
	}
	return chosen, true
}

// persistDataDir remembers the resolved vault so later launches skip
// onboarding. A write failure is non-fatal; the app still starts.
func (d *desktop) persistDataDir(dataDir string) {
	if err := config.SaveDesktopConfig(config.DesktopConfig{DataDir: dataDir}); err != nil {
		d.logger.Warn("save desktop config", "error", err)
	}
}

// desktopHooks exposes shell state to the local HTTP API. The server depends
// only on its methods, never on Wails.
type desktopHooks struct {
	desktop *desktop
}

func (h *desktopHooks) AutostartEnabled() bool {
	enabled, err := h.desktop.wails.Autostart.IsEnabled()
	if err != nil {
		h.desktop.logger.Warn("read autostart state", "error", err)
		return false
	}
	return enabled
}

func (h *desktopHooks) SetAutostart(on bool) error {
	if on {
		return h.desktop.wails.Autostart.Enable()
	}
	return h.desktop.wails.Autostart.Disable()
}

func (h *desktopHooks) DataDir() string { return h.desktop.cfg.DataDir }

func (h *desktopHooks) SetVault(dataDir string) (string, bool, error) {
	return h.desktop.changeVault(dataDir)
}

func (h *desktopHooks) Restart() error {
	return h.desktop.restart()
}

func (h *desktopHooks) OpenDataDir() error {
	return h.desktop.wails.Env.OpenFileManager(h.desktop.cfg.DataDir, false)
}

// desktopSyncFactory builds the vault sync engine from the app's local vault.
// The engine also serves the desktop sync endpoints through app.Options.
func desktopSyncFactory(deps app.SyncDeps) (app.SyncEngine, error) {
	return overviewsync.NewEngine(overviewsync.Options{
		DataDir:   deps.DataDir,
		Repo:      deps.Repo,
		Raw:       deps.Raw,
		Assets:    deps.Assets,
		Logger:    deps.Logger,
		UserAgent: "Overview-Desktop/" + version,
	})
}

// forwardArgs routes a second instance's command line, which carries a file
// path or an overview:// URL when the OS opens a document or deep link.
func (d *desktop) forwardArgs(args []string) {
	for _, arg := range args[1:] {
		arg = strings.TrimSpace(arg)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		d.handleOpen(arg)
	}
}

// handleOpen opens a file path or deep link, queueing it until the window is
// ready. It is safe to call from any goroutine and at any point in the
// lifecycle.
func (d *desktop) handleOpen(target string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return
	}
	if !d.ready() {
		d.mu.Lock()
		d.pending = append(d.pending, target)
		d.mu.Unlock()
		return
	}
	if isDeepLink(target) {
		d.openDeepLink(target)
		return
	}
	d.openFile(target)
}

func (d *desktop) ready() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.server != nil && d.window != nil && d.baseURL != ""
}

func (d *desktop) drainPending() {
	d.mu.Lock()
	pending := d.pending
	d.pending = nil
	d.mu.Unlock()
	for _, target := range pending {
		d.handleOpen(target)
	}
}

// openFile resolves an external Markdown file to a vault path, importing it
// when it lives outside the vault, then navigates the window to it.
func (d *desktop) openFile(path string) {
	d.mu.Lock()
	server := d.server
	d.mu.Unlock()
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rel, err := server.ResolveOrImport(ctx, path)
	if err != nil {
		d.reportProblem("无法打开文件", fmt.Sprintf("无法打开 %s：%s", path, err))
		return
	}
	d.openNote(rel)
}

// isDeepLink reports whether target is an overview:// URL.
func isDeepLink(target string) bool {
	return strings.HasPrefix(strings.ToLower(target), "overview://")
}

// openDeepLink parses an overview:// URL and opens the note it names. Unknown
// forms are reported to the user.
func (d *desktop) openDeepLink(raw string) {
	rel, ok := parseDeepLink(raw)
	if !ok {
		d.reportProblem("无法打开链接", "不支持该链接：\n"+raw)
		return
	}
	d.openNote(rel)
}

// parseDeepLink understands overview://note/<path> and
// overview://open?path=<path>.
func parseDeepLink(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "overview") {
		return "", false
	}
	switch strings.ToLower(u.Host) {
	case "note":
		if p := strings.Trim(u.Path, "/"); p != "" {
			return p, true
		}
		if p := strings.Trim(u.Query().Get("path"), "/"); p != "" {
			return p, true
		}
	case "open":
		if p := strings.Trim(u.Query().Get("path"), "/"); p != "" {
			return p, true
		}
	}
	return "", false
}

// openNote navigates the window to a note route. The ?desktop=1 marker is
// preserved so the SPA keeps detecting the shell after a reload.
func (d *desktop) openNote(rel string) {
	rel = strings.Trim(strings.TrimSpace(rel), "/")
	if rel == "" {
		return
	}
	d.mu.Lock()
	window, base := d.window, d.baseURL
	d.mu.Unlock()
	if window == nil || base == "" {
		return
	}
	window.SetURL(base + "/note/" + encodeRelPath(rel) + "?desktop=1")
	d.showWindow()
}

// encodeRelPath escapes each segment of a slash-separated path while keeping
// the separators, so nested notes stay reachable.
func encodeRelPath(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// startUpdateChecks polls GitHub for a newer release shortly after launch and
// then once a day, notifying the user when one is found.
func (d *desktop) startUpdateChecks() {
	go func() {
		select {
		case <-time.After(5 * time.Second):
		case <-d.wails.Context().Done():
			return
		}
		d.checkForUpdate()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.checkForUpdate()
			case <-d.wails.Context().Done():
				return
			}
		}
	}()
}

func (d *desktop) checkForUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	latest, url, hasUpdate, err := update.CheckLatest(ctx, version)
	if err != nil {
		d.logger.Debug("update check failed", "error", err)
		return
	}
	if !hasUpdate {
		return
	}
	d.mu.Lock()
	if d.lastNotified == latest {
		d.mu.Unlock()
		return
	}
	d.lastNotified = latest
	d.mu.Unlock()

	err = d.notifier.SendNotification(notifications.NotificationOptions{
		ID:    "overview-update-" + latest,
		Title: "Overview 有新版本",
		Body:  "发现新版本 " + latest + "，点击查看。",
		Data:  map[string]any{"url": url},
	})
	if err != nil {
		d.logger.Warn("send update notification", "error", err)
	}
}

// setupTray installs the tray icon and its menu.
func (d *desktop) setupTray() {
	tray := d.wails.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		tray.SetIcon(iconPNG)
	}
	tray.SetTooltip("Overview")

	menu := d.wails.NewMenu()
	menu.Add("打开 Overview").OnClick(func(*application.Context) { d.showWindow() })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { d.quit() })
	tray.SetMenu(menu)
	tray.OnClick(func() { d.showWindow() })
}

// showWindow restores and focuses the window. It is safe to call before the
// window exists, for example when a second instance launches during startup.
func (d *desktop) showWindow() {
	d.mu.Lock()
	window := d.window
	d.mu.Unlock()
	if window == nil {
		return
	}
	window.Show()
	window.Restore()
	window.Focus()
}

// quit stops the local server and exits the Wails application.
func (d *desktop) quit() {
	d.stopServer()
	d.wails.Quit()
}

// stopServer gracefully shuts the shared server down. It is idempotent and is
// wired into the Wails shutdown sequence as well as the tray menu.
func (d *desktop) stopServer() {
	d.mu.Lock()
	server := d.server
	d.mu.Unlock()
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		d.logger.Warn("stop local server", "error", err)
	}
}

// fatal reports a startup failure and exits.
func (d *desktop) fatal(title string, err error) {
	d.logger.Error(title, "error", err)
	d.sendProblemNotification(title, err.Error())
	d.quit()
}

// reportProblem surfaces a failed file or deep-link open without blocking the
// caller. The second-instance launch path runs on a single listener goroutine
// that also feeds the OS single-instance channel; parking it on a modal dialog
// would stall later launches (and hang the second process), so the notice is
// logged immediately and delivered as a system notification.
func (d *desktop) reportProblem(title, message string) {
	d.logger.Warn("desktop open request failed", "title", title, "detail", message)
	if d.problemReporter != nil {
		d.problemReporter(title, message)
		return
	}
	go d.sendProblemNotification(title, message)
}

// sendProblemNotification raises a system notification describing a failure.
// It is safe to call when no notifier is configured.
func (d *desktop) sendProblemNotification(title, message string) {
	notifier := d.notifier
	if notifier == nil {
		return
	}
	err := notifier.SendNotification(notifications.NotificationOptions{
		ID:    "overview-open-problem",
		Title: title,
		Body:  message,
	})
	if err != nil {
		d.logger.Warn("send problem notification", "error", err)
	}
}

// cliFlags is the result of parsing the desktop command line.
type cliFlags struct {
	cfg             config.Config
	explicitDataDir bool
}

// parseFlags applies --data-dir/-C, --addr and --auth on top of the
// environment-derived configuration. An explicit data directory suppresses the
// first-run picker.
func parseFlags(args []string, base config.Config) cliFlags {
	flags := cliFlags{cfg: base, explicitDataDir: os.Getenv("OVERVIEW_DATA_DIR") != ""}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--data-dir" || arg == "-C":
			if i+1 < len(args) {
				flags.cfg = flags.cfg.WithDataDir(args[i+1])
				flags.explicitDataDir = true
				i++
			}
		case strings.HasPrefix(arg, "--data-dir="):
			flags.cfg = flags.cfg.WithDataDir(strings.TrimPrefix(arg, "--data-dir="))
			flags.explicitDataDir = true
		case arg == "--addr":
			if i+1 < len(args) {
				flags.cfg.Addr = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--addr="):
			flags.cfg.Addr = strings.TrimPrefix(arg, "--addr=")
		case arg == "--auth":
			if i+1 < len(args) {
				flags.cfg.AuthMode = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--auth="):
			flags.cfg.AuthMode = strings.TrimPrefix(arg, "--auth=")
		}
	}
	return flags
}

// needsBootstrap reports whether the directory is missing or empty, meaning the
// user has no vault yet and should be asked where to keep it.
func needsBootstrap(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	return len(entries) == 0
}

// desktopBaseURL builds the loopback origin the webview should open.
func desktopBaseURL(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	return "http://127.0.0.1:" + port, nil
}

// desktopURL builds the loopback URL the webview should open from a resolved
// origin and the build version. The ?desktop=1 marker lets the frontend detect
// the shell (it also skips service worker registration); the ?v= version gives
// every release a fresh WebView2 cache entry so an upgrade loads the new
// frontend bundle instead of a stale index.html.
func desktopURL(base, version string) string {
	return base + "/?desktop=1&v=" + url.QueryEscape(version)
}

// ensureDesktopEnv defaults the process to desktop mode so config.Load applies
// the desktop defaults (loopback address, no auth, MCP/WebDAV off).
func ensureDesktopEnv() {
	if os.Getenv("OVERVIEW_DESKTOP") == "" {
		_ = os.Setenv("OVERVIEW_DESKTOP", "true")
	}
}

// setupLogger builds the application logger shared with the local server.
func setupLogger(cfg config.Config) *logging.Logger {
	lg, err := logging.New(logging.Options{
		Level:   cfg.LogLevel,
		Format:  cfg.LogFormat,
		File:    desktopLogFile(cfg),
		MaxMB:   cfg.LogMaxMB,
		Backups: cfg.LogBackups,
	})
	if err != nil {
		lg, _ = logging.New(logging.Options{Level: cfg.LogLevel, Format: cfg.LogFormat})
	}
	slog.SetDefault(lg.Logger)
	return lg
}

// desktopLogFile returns the file the desktop shell logs to. An explicit
// OVERVIEW_LOG_FILE (surfaced through cfg.LogFile) always wins; otherwise logs
// go to <DataDir>/logs/desktop.log. The GUI build has no console, so a file is
// the only place to diagnose a failed launch.
func desktopLogFile(cfg config.Config) string {
	if cfg.LogFile != "" {
		return cfg.LogFile
	}
	return filepath.Join(cfg.DataDir, "logs", "desktop.log")
}
