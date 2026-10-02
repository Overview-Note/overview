// Package server is the HTTP adapter layer. Handlers are intentionally thin:
// they decode input, delegate to the service layer and map domain errors to
// HTTP responses.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/service"
)

// Options configures a Server.
type Options struct {
	MaxUploadBytes int64
	Static         fs.FS
	Version        string
	Logger         *slog.Logger
	Auth           *service.AuthService
	DAV            http.Handler
	MCP            http.Handler
	AI             *service.AIService
	// Render runs the server as a public documentation site: no auth, the
	// note tree is exposed read-only and the SPA renders notes directly.
	Render    bool
	SiteTitle string
}

// Server routes HTTP requests to the application service.
type Server struct {
	svc    *service.Service
	opts   Options
	logger *slog.Logger
	mux    *http.ServeMux
}

// New constructs a Server.
func New(svc *service.Service, opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Auth == nil {
		opts.Auth = service.NewAuth(nil, "none")
	}
	if opts.Render {
		// A documentation site is fully public and read-only.
		opts.Auth = service.NewAuth(nil, "none")
	}
	s := &Server{svc: svc, opts: opts, logger: opts.Logger, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the composed handler (middleware + routes).
func (s *Server) Handler() http.Handler {
	return requestID(recoverer(s.logger)(requestLogger(s.logger)(s.renderGuard(s.authMiddleware(s.mux)))))
}

// renderGuard rejects state-changing API requests when running as a docs site.
func (s *Server) renderGuard(next http.Handler) http.Handler {
	if !s.opts.Render {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusMethodNotAllowed, "read-only site")
				return
			}
			next.ServeHTTP(w, r)
		}
	})
}

func (s *Server) routes() {
	const base = "/api/v1"

	// Authentication.
	s.mux.HandleFunc("GET "+base+"/auth/state", s.handleAuthState)
	s.mux.HandleFunc("POST "+base+"/auth/setup", s.handleSetup)
	s.mux.HandleFunc("POST "+base+"/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST "+base+"/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET "+base+"/auth/me", s.handleMe)
	s.mux.HandleFunc("POST "+base+"/auth/password", s.handleChangePassword)
	s.mux.HandleFunc("GET "+base+"/auth/users", s.handleListUsers)
	s.mux.HandleFunc("POST "+base+"/auth/users", s.handleCreateUser)
	s.mux.HandleFunc("DELETE "+base+"/auth/users/{id}", s.handleDeleteUser)

	s.mux.HandleFunc("GET "+base+"/health", s.handleHealth)
	s.mux.HandleFunc("GET "+base+"/tree", s.handleTree)
	s.mux.HandleFunc("GET "+base+"/note", s.handleGetNote)
	s.mux.HandleFunc("PUT "+base+"/note", s.handleSaveNote)
	s.mux.HandleFunc("DELETE "+base+"/note", s.handleDelete)
	s.mux.HandleFunc("POST "+base+"/folder", s.handleCreateFolder)
	s.mux.HandleFunc("POST "+base+"/rename", s.handleRename)
	s.mux.HandleFunc("GET "+base+"/search", s.handleSearch)
	s.mux.HandleFunc("GET "+base+"/links", s.handleLinks)
	s.mux.HandleFunc("GET "+base+"/resolve", s.handleResolve)
	s.mux.HandleFunc("POST "+base+"/assets", s.handleUploadAsset)
	s.mux.HandleFunc("GET "+base+"/assets/{path...}", s.handleAsset)
	s.mux.HandleFunc("POST "+base+"/reindex", s.handleReindex)

	// API documentation (public).
	s.mux.HandleFunc("GET /api/v1/openapi.json", s.handleOpenAPI)
	s.mux.HandleFunc("GET /api/docs", s.handleAPIDocs)

	// Asset maintenance.
	s.mux.HandleFunc("GET "+base+"/assets/orphans", s.handleOrphanAssets)
	s.mux.HandleFunc("POST "+base+"/assets/orphans/purge", s.handlePurgeOrphans)

	// Version history.
	s.mux.HandleFunc("GET "+base+"/history", s.handleHistoryList)
	s.mux.HandleFunc("GET "+base+"/history/revision", s.handleHistoryGet)
	s.mux.HandleFunc("POST "+base+"/history/restore", s.handleHistoryRestore)

	// Trash.
	s.mux.HandleFunc("GET "+base+"/trash", s.handleTrashList)
	s.mux.HandleFunc("POST "+base+"/trash/restore", s.handleTrashRestore)
	s.mux.HandleFunc("DELETE "+base+"/trash", s.handleTrashPurge)

	// AI assistance (available only when configured).
	s.mux.HandleFunc("GET "+base+"/ai/status", s.handleAIStatus)
	s.mux.HandleFunc("POST "+base+"/ai/chat", s.handleAIChat)
	s.mux.HandleFunc("GET "+base+"/settings/ai", s.handleGetAISettings)
	s.mux.HandleFunc("PUT "+base+"/settings/ai", s.handleSaveAISettings)

	// Public (anonymous) read-only access to shared notes.
	s.mux.HandleFunc("GET "+base+"/public/notes", s.handlePublicNotes)
	s.mux.HandleFunc("GET "+base+"/public/note", s.handlePublicNote)

	// Vault-relative asset URLs (portable across tools / exports).
	s.mux.HandleFunc("GET /assets/{path...}", s.handleAsset)

	if s.opts.DAV != nil {
		s.mux.Handle("/dav/", s.opts.DAV)
		s.mux.Handle("/dav", s.opts.DAV)
	}
	if s.opts.MCP != nil {
		s.mux.Handle("/mcp", s.opts.MCP)
	}

	s.mountStatic()
}

func (s *Server) mountStatic() {
	if s.opts.Static == nil {
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			writeError(w, http.StatusNotFound, "frontend not built")
		})
		return
	}
	fileServer := http.FileServer(http.FS(s.opts.Static))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.opts.Static, p); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// middleware
// ---------------------------------------------------------------------------

type ctxKey int

const ctxRequestID ctxKey = iota

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = time.Now().UTC().Format("20060102150405.000000")
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", r.Context().Value(ctxRequestID),
			)
		})
	}
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "error", rec, "path", r.URL.Path)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// authentication
// ---------------------------------------------------------------------------

const sessionCookie = "overview_session"

type userCtxKey int

const ctxUser userCtxKey = iota

func withUser(ctx context.Context, u core.User) context.Context {
	return context.WithValue(ctx, ctxUser, u)
}

func userFromContext(ctx context.Context) (core.User, bool) {
	u, ok := ctx.Value(ctxUser).(core.User)
	return u, ok
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.opts.Auth.Required() {
			next.ServeHTTP(w, r)
			return
		}
		p := r.URL.Path
		if isPublicPath(p) {
			next.ServeHTTP(w, r)
			return
		}
		if !requiresAuth(p, s.opts.Static) {
			next.ServeHTTP(w, r) // SPA shell and bundled assets are public
			return
		}
		user, err := s.opts.Auth.Authenticate(r.Context(), s.sessionToken(r))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

func isPublicPath(p string) bool {
	switch p {
	case "/api/v1/health",
		"/api/v1/auth/state",
		"/api/v1/auth/setup",
		"/api/v1/auth/login",
		"/api/v1/public/notes",
		"/api/v1/public/note":
		return true
	default:
		return false
	}
}

// requiresAuth reports whether a request path must be authenticated. API and
// uploaded attachment paths do; the SPA shell and embedded static assets do
// not, and the WebDAV handler performs its own Basic auth.
func requiresAuth(p string, static fs.FS) bool {
	switch {
	case strings.HasPrefix(p, "/dav"):
		return false
	case p == "/mcp":
		return false // MCP performs its own bearer-token auth
	case p == "/api/v1/openapi.json", p == "/api/docs":
		return false
	case strings.HasPrefix(p, "/api/v1/"):
		return true
	case strings.HasPrefix(p, "/assets/"):
		// A real static file wins over the attachment route.
		if static != nil {
			if _, err := fs.Stat(static, strings.TrimPrefix(p, "/")); err == nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (s *Server) sessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}

// ---------------------------------------------------------------------------
// responses
// ---------------------------------------------------------------------------

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: statusCode(status), Message: msg}})
}

func statusCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal"
	}
}

// writeDomainError maps domain sentinel errors to HTTP status codes.
func (s *Server) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, core.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, core.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, core.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, core.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		s.logger.Error("internal error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
