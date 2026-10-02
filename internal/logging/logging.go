// Package logging builds the application logger. It writes structured logs to
// stdout and, optionally, to a rotating file.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Options configures the logger.
type Options struct {
	Level   string // debug | info | warn | error
	Format  string // json | text
	File    string // optional log file path; empty logs to stdout only
	MaxMB   int    // rotate after this many megabytes (default 10)
	Backups int    // rotated files to keep; 0 truncates instead
}

// Logger wraps slog.Logger and the file handle, if any.
type Logger struct {
	*slog.Logger
	closer io.Closer
}

// New builds a Logger from options.
func New(opts Options) (*Logger, error) {
	level := ParseLevel(opts.Level)

	var w io.Writer = os.Stdout
	var closer io.Closer
	if opts.File != "" {
		if dir := filepath.Dir(opts.File); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create log dir: %w", err)
			}
		}
		r, err := newRotator(opts.File, opts.MaxMB, opts.Backups)
		if err != nil {
			return nil, err
		}
		closer = r
		w = io.MultiWriter(os.Stdout, r)
	}

	slogOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch strings.ToLower(opts.Format) {
	case "text":
		handler = slog.NewTextHandler(w, slogOpts)
	default:
		handler = slog.NewJSONHandler(w, slogOpts)
	}
	return &Logger{Logger: slog.New(handler), closer: closer}, nil
}

// Close flushes and closes the log file, if any.
func (l *Logger) Close() error {
	if l == nil || l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

// ParseLevel maps a level name to slog.Level (defaults to info).
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// rotator is a size-based rotating file writer.
type rotator struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	file     *os.File
	size     int64
}

func newRotator(path string, maxMB, backups int) (*rotator, error) {
	if maxMB <= 0 {
		maxMB = 10
	}
	if backups < 0 {
		backups = 0
	}
	r := &rotator{path: path, maxBytes: int64(maxMB) << 20, backups: backups}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rotator) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	r.file = f
	r.size = info.Size()
	return nil
}

func (r *rotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size+int64(len(p)) > r.maxBytes {
		r.rotate()
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *rotator) rotate() {
	_ = r.file.Close()
	if r.backups == 0 {
		r.file, _ = os.OpenFile(r.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		r.size = 0
		return
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", r.path, r.backups))
	for i := r.backups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", r.path, i), fmt.Sprintf("%s.%d", r.path, i+1))
	}
	_ = os.Rename(r.path, r.path+".1")
	if err := r.open(); err != nil {
		// Last resort: keep writing without file logging.
		r.file = os.Stderr
		r.size = 0
	}
}

func (r *rotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}
