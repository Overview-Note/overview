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
	dir string
}

// New creates a history store rooted at dir (e.g. <data>/.history).
func New(dir string) *Store { return &Store{dir: dir} }

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
	return Revision{
		ID:      id,
		Path:    filepath.ToSlash(rel),
		SavedAt: time.Now().UTC(),
		Size:    int64(len(raw)),
		Version: version,
	}, nil
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
