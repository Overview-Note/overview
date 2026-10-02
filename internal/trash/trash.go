// Package trash stores soft-deleted notes so they can be restored.
package trash

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/Overview-Note/overview/internal/core"
)

// Entry describes a trashed note or folder.
type Entry struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	IsDir     bool      `json:"isDir"`
	DeletedAt time.Time `json:"deletedAt"`
	Size      int64     `json:"size"`
	Original  string    `json:"original"`
}

// Store holds trashed items on the filesystem.
type Store struct {
	dir string
}

// New creates a trash store rooted at dir.
func New(dir string) *Store { return &Store{dir: dir} }

// Move relocates a note/folder at src into the trash, preserving its origin.
func (s *Store) Move(src, rel string, isDir bool) (Entry, error) {
	id := ulid.Make().String()
	dest := filepath.Join(s.dir, id)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Entry{}, err
	}
	target := filepath.Join(dest, filepath.Base(filepath.FromSlash(rel)))
	if err := os.Rename(src, target); err != nil {
		return Entry{}, err
	}
	entry := Entry{
		ID:        id,
		Path:      filepath.ToSlash(rel),
		IsDir:     isDir,
		DeletedAt: time.Now().UTC(),
		Original:  filepath.ToSlash(rel),
	}
	if info, err := os.Stat(target); err == nil {
		entry.Size = info.Size()
	}
	if err := s.writeMeta(id, entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// List returns all trashed entries, newest first.
func (s *Store) List() ([]Entry, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if entry, err := s.readMeta(e.Name()); err == nil {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DeletedAt.After(out[j].DeletedAt)
	})
	return out, nil
}

// Restore moves a trashed item back to destPath, which must be an absolute
// filesystem path (typically inside the notes directory).
func (s *Store) Restore(id, destPath string) error {
	src, err := s.TrashPath(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.Rename(src, destPath)
}

// Meta returns the metadata of a trashed entry.
func (s *Store) Meta(id string) (Entry, error) { return s.readMeta(id) }

// Purge permanently removes a trashed entry.
func (s *Store) Purge(id string) error {
	if _, err := os.Stat(filepath.Join(s.dir, id)); os.IsNotExist(err) {
		return core.ErrNotFound
	}
	return os.RemoveAll(filepath.Join(s.dir, id))
}

func (s *Store) writeMeta(id string, entry Entry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, id, "meta.json"), b, 0o644)
}

func (s *Store) readMeta(id string) (Entry, error) {
	var entry Entry
	b, err := os.ReadFile(filepath.Join(s.dir, id, "meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, core.ErrNotFound
		}
		return Entry{}, err
	}
	if err := json.Unmarshal(b, &entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// TrashPath returns the filesystem path of the trashed payload.
func (s *Store) TrashPath(id string) (string, error) {
	entry, err := s.readMeta(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.dir, id, filepath.Base(filepath.FromSlash(entry.Path))), nil
}

// IsTrashed reports whether id already exists in the trash.
func (s *Store) IsTrashed(id string) bool {
	_, err := os.Stat(filepath.Join(s.dir, id))
	return err == nil
}
