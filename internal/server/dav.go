package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/webdav"

	"github.com/Overview-Note/overview/internal/service"
)

// NewDAV builds a WebDAV handler that exposes the notes directory. When
// authentication is required it uses HTTP Basic auth against the user store.
// Any successful write schedules a debounced reindex so the search index
// stays in sync with external edits.
func NewDAV(notesDir string, auth *service.AuthService, reindex func(), logger *slog.Logger) http.Handler {
	handler := &webdav.Handler{
		Prefix:     "/dav",
		FileSystem: webdav.Dir(notesDir),
		LockSystem: webdav.NewMemLS(),
	}
	trigger := &reindexer{fn: reindex, logger: logger}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth != nil && auth.Required() {
			user, pass, ok := r.BasicAuth()
			if !ok {
				unauthorized(w)
				return
			}
			if _, err := auth.VerifyPassword(r.Context(), user, pass); err != nil {
				unauthorized(w)
				return
			}
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		handler.ServeHTTP(rec, r)
		if rec.status < http.StatusBadRequest && isWriteMethod(r.Method) {
			trigger.schedule()
		}
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Overview", charset="UTF-8"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPut, http.MethodDelete, http.MethodPost,
		"MKCOL", "MOVE", "COPY", "PROPPATCH", "LOCK", "UNLOCK":
		return true
	default:
		return false
	}
}

type reindexer struct {
	mu     sync.Mutex
	timer  *time.Timer
	fn     func()
	logger *slog.Logger
}

func (r *reindexer) schedule() {
	if r.fn == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(time.Second, func() {
		r.fn()
		if r.logger != nil {
			r.logger.Info("reindexed after WebDAV write")
		}
	})
}
