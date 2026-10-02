// Package history stores previous revisions of notes as files, keeping the
// "files as source of truth" principle. Snapshots live under a hidden
// .history directory so the notes tree walker ignores them.
package history

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/Overview-Note/overview/internal/core"
)

// Revision describes a stored snapshot.
type Revision struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	SavedAt time.Time `json:"savedAt"`
	Size    int64     `json:"size"`
	Version string    `json:"version"`
}

// Store persists note revisions on the filesystem.
type Store struct {
	dir  string
	keep int // max revisions per note; <=0 keeps everything
}

// New creates a history store rooted at dir (e.g. <data>/.history). keep is the
// maximum number of revisions retained per note (0 disables pruning).
func New(dir string, keep int) *Store { return &Store{dir: dir, keep: keep} }

// Snapshot saves raw content of rel as a revision and returns it.
func (s *Store) Snapshot(rel, version string, raw []byte) (Revision, error) {
	if len(raw) == 0 {
		return Revision{}, nil
	}
	dir := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Revision{}, err
	}
	id := ulid.Make().String()
	name := time.Now().UTC().Format("20060102T150405Z") + "-" + id + ".md"
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		return Revision{}, err
	}
	s.prune(dir)
	return Revision{
		ID:      id,
		Path:    filepath.ToSlash(rel),
		SavedAt: time.Now().UTC(),
		Size:    int64(len(raw)),
		Version: version,
	}, nil
}

// prune keeps only the newest s.keep snapshots in dir, deleting the rest.
func (s *Store) prune(dir string) {
	if s.keep <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) <= s.keep {
		return
	}
	type snap struct {
		name string
		mod  time.Time
	}
	snaps := make([]snap, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		snaps = append(snaps, snap{name: e.Name(), mod: info.ModTime()})
	}
	if len(snaps) <= s.keep {
		return
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].mod.After(snaps[j].mod) })
	for _, old := range snaps[s.keep:] {
		_ = os.Remove(filepath.Join(dir, old.name))
	}
}

// List returns revisions for rel, newest first.
func (s *Store) List(rel string) ([]Revision, error) {
	dir := filepath.Join(s.dir, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Revision{}, nil
		}
		return nil, err
	}
	revisions := make([]Revision, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		id := base
		if i := strings.LastIndexByte(base, '-'); i >= 0 {
			id = base[i+1:]
		}
		revisions = append(revisions, Revision{
			ID:      id,
			Path:    filepath.ToSlash(rel),
			SavedAt: info.ModTime().UTC(),
			Size:    info.Size(),
		})
	}
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].SavedAt.After(revisions[j].SavedAt)
	})
	return revisions, nil
}

// Content returns the raw content of a revision identified by id.
func (s *Store) Content(rel, id string) ([]byte, error) {
	dir := filepath.Join(s.dir, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		if strings.HasSuffix(base, "-"+id) {
			return os.ReadFile(filepath.Join(dir, e.Name()))
		}
	}
	return nil, core.ErrNotFound
}
