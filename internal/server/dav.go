package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"

	"github.com/Overview-Note/overview/internal/service"
)

// NewDAV builds a WebDAV handler that exposes the notes directory. When
// authentication is required it uses HTTP Basic auth against the user store.
// Successful writes record the affected paths and, after a short debounce,
// reindex only those subtrees so the search index stays in sync with external
// edits without a full rebuild.
func NewDAV(notesDir string, auth *service.AuthService, reindex func(paths []string), logger *slog.Logger) http.Handler {
	fs := &recordingFS{FileSystem: webdav.Dir(notesDir), paths: map[string]struct{}{}}
	handler := &webdav.Handler{
		Prefix:     "/dav",
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
	}
	trigger := &reindexer{fn: reindex, drain: fs.drain, logger: logger}

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

// recordingFS wraps a WebDAV filesystem and remembers the vault-relative paths
// touched by writes.
type recordingFS struct {
	webdav.FileSystem
	mu    sync.Mutex
	paths map[string]struct{}
}

func (f *recordingFS) record(name string) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		return
	}
	f.mu.Lock()
	f.paths[name] = struct{}{}
	f.mu.Unlock()
}

func (f *recordingFS) drain() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.paths))
	for p := range f.paths {
		out = append(out, p)
	}
	f.paths = map[string]struct{}{}
	return out
}

func (f *recordingFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	err := f.FileSystem.Mkdir(ctx, name, perm)
	if err == nil {
		f.record(name)
	}
	return err
}

func (f *recordingFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := f.FileSystem.OpenFile(ctx, name, flag, perm)
	if err == nil && flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		f.record(name)
	}
	return file, err
}

func (f *recordingFS) RemoveAll(ctx context.Context, name string) error {
	err := f.FileSystem.RemoveAll(ctx, name)
	if err == nil {
		f.record(name)
	}
	return err
}

func (f *recordingFS) Rename(ctx context.Context, oldName, newName string) error {
	err := f.FileSystem.Rename(ctx, oldName, newName)
	if err == nil {
		f.record(oldName)
		f.record(newName)
	}
	return err
}

type reindexer struct {
	mu     sync.Mutex
	timer  *time.Timer
	fn     func([]string)
	drain  func() []string
	logger *slog.Logger
}

func (r *reindexer) schedule() {
	if r.fn == nil || r.drain == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(time.Second, func() {
		paths := r.drain()
		if len(paths) == 0 {
			return
		}
		r.fn(paths)
		if r.logger != nil {
			r.logger.Info("reindexed after WebDAV write", "paths", len(paths))
		}
	})
}
