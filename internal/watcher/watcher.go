// Package watcher monitors the notes directory for external changes (editors,
// Git pulls, WebDAV) and reindexes the affected paths so search stays in sync.
package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Reindexer reindexes a subtree rooted at a vault-relative path.
type Reindexer interface {
	ReindexPath(ctx context.Context, path string) error
}

// Watcher watches a directory tree and triggers debounced reindexing.
type Watcher struct {
	root     string
	reindex  Reindexer
	logger   *slog.Logger
	debounce time.Duration

	mu      sync.Mutex
	pending map[string]struct{}
	timer   *time.Timer
}

// New creates a Watcher rooted at notesDir.
func New(notesDir string, reindex Reindexer, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		root:     notesDir,
		reindex:  reindex,
		logger:   logger,
		debounce: 500 * time.Millisecond,
		pending:  map[string]struct{}{},
	}
}

// Run watches until ctx is cancelled, blocking until the watcher stops.
func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	if err := w.addRecursive(fsw, w.root); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			// A new directory appeared: watch it too.
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.addRecursive(fsw, event.Name)
				}
			}
			if rel, ok := w.rel(event.Name); ok && isRelevant(rel) {
				w.schedule(rel)
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			w.logger.Warn("watcher error", "error", err)
		}
	}
}

func (w *Watcher) addRecursive(fsw *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != w.root {
				return filepath.SkipDir
			}
			return fsw.Add(path)
		}
		return nil
	})
}

func (w *Watcher) rel(name string) (string, bool) {
	rel, err := filepath.Rel(w.root, name)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func isRelevant(rel string) bool {
	base := rel
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		base = rel[i+1:]
	}
	if strings.HasPrefix(base, ".") {
		return false
	}
	// Temp files created by atomic writes.
	if strings.HasPrefix(base, "overview-tmp-") || strings.HasPrefix(base, ".overview-tmp-") {
		return false
	}
	return true
}

func (w *Watcher) schedule(rel string) {
	top := rel
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		top = rel[:i]
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending[top] = struct{}{}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.debounce, w.flush)
}

func (w *Watcher) flush() {
	w.mu.Lock()
	pending := w.pending
	w.pending = map[string]struct{}{}
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for path := range pending {
		if err := w.reindex.ReindexPath(ctx, path); err != nil {
			w.logger.Warn("reindex after external change failed", "path", path, "error", err)
			continue
		}
		w.logger.Info("reindexed after external change", "path", path)
	}
}
