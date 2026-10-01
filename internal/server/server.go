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

	"github.com/overview-app/overview/internal/core"
	"github.com/overview-app/overview/internal/service"
)

// Options configures a Server.
type Options struct {
	MaxUploadBytes int64
	Static         fs.FS
	Version        string
	Logger         *slog.Logger
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
	s := &Server{svc: svc, opts: opts, logger: opts.Logger, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the composed handler (middleware + routes).
func (s *Server) Handler() http.Handler {
	return requestID(recoverer(s.logger)(requestLogger(s.logger)(s.mux)))
}

func (s *Server) routes() {
	const base = "/api/v1"
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

	// Vault-relative asset URLs (portable across tools / exports).
	s.mux.HandleFunc("GET /assets/{path...}", s.handleAsset)

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
	default:
		s.logger.Error("internal error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
