// Package store implements core.NoteRepository and core.AssetStore on top of
// the local filesystem. Notes are stored as Markdown files with YAML
// frontmatter; folders map directly to directories.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/overview-app/overview/internal/core"
	"github.com/overview-app/overview/internal/markdown"
)

// Store is a filesystem-backed note and asset repository.
type Store struct {
	notesDir  string
	assetsDir string
}

var (
	_ core.NoteRepository = (*Store)(nil)
	_ core.AssetStore     = (*Store)(nil)
)

// New creates a Store rooted at the given notes and assets directories.
func New(notesDir, assetsDir string) *Store {
	return &Store{notesDir: notesDir, assetsDir: assetsDir}
}

// NotesDir exposes the notes root.
func (s *Store) NotesDir() string { return s.notesDir }

// AssetsDir exposes the assets root.
func (s *Store) AssetsDir() string { return s.assetsDir }

// ---------------------------------------------------------------------------
// NoteRepository
// ---------------------------------------------------------------------------

// Read loads and parses the note at rel.
func (s *Store) Read(_ context.Context, rel string) (core.Note, error) {
	full, err := resolve(s.notesDir, rel)
	if err != nil {
		return core.Note{}, err
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return core.Note{}, core.ErrNotFound
		}
		return core.Note{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return core.Note{}, err
	}
	return buildNote(toSlashPath(rel), raw, info), nil
}

// Raw returns the raw bytes of the note file at rel.
func (s *Store) Raw(_ context.Context, rel string) ([]byte, error) {
	full, err := resolve(s.notesDir, rel)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return raw, nil
}

// Write creates or updates a note according to the version contract documented
// on core.NoteRepository.
func (s *Store) Write(_ context.Context, rel, body, expectedVersion string, public bool) (core.Note, error) {
	if !isMarkdown(rel) {
		rel += ".md"
	}
	full, err := resolve(s.notesDir, rel)
	if err != nil {
		return core.Note{}, err
	}

	var (
		meta    markdown.Frontmatter
		exists  bool
		curVers string
	)
	if raw, err := os.ReadFile(full); err == nil {
		exists = true
		curVers = hashBytes(raw)
		meta = markdown.Parse(string(raw)).Meta
	} else if !os.IsNotExist(err) {
		return core.Note{}, err
	}

	if err := checkVersion(expectedVersion, exists, curVers); err != nil {
		return core.Note{}, err
	}

	now := time.Now().UTC()
	if meta.ID == "" {
		meta.ID = ulid.Make().String()
	}
	if meta.Created == "" {
		meta.Created = now.Format(time.RFC3339)
	}
	meta.Updated = now.Format(time.RFC3339)
	meta.Public = public
	if meta.Title == "" {
		meta.Title = deriveTitle(filepath.Base(rel), body)
	}

	doc := markdown.Document{Meta: meta, Body: body}
	if err := atomicWrite(full, []byte(doc.String())); err != nil {
		return core.Note{}, err
	}

	info, err := os.Stat(full)
	if err != nil {
		return core.Note{}, err
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return core.Note{}, err
	}
	return buildNote(toSlashPath(rel), raw, info), nil
}

// Delete removes a note or folder recursively.
func (s *Store) Delete(_ context.Context, rel string) error {
	full, err := resolve(s.notesDir, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(full); os.IsNotExist(err) {
		return core.ErrNotFound
	}
	return os.RemoveAll(full)
}

// Move renames or relocates a note or folder.
func (s *Store) Move(_ context.Context, from, to string) error {
	oldFull, err := resolve(s.notesDir, from)
	if err != nil {
		return err
	}
	newFull, err := resolve(s.notesDir, to)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldFull); os.IsNotExist(err) {
		return core.ErrNotFound
	}
	if _, err := os.Stat(newFull); err == nil {
		return core.Conflictf("destination already exists: %s", to)
	}
	if err := os.MkdirAll(filepath.Dir(newFull), 0o755); err != nil {
		return err
	}
	return os.Rename(oldFull, newFull)
}

// Mkdir creates a folder (and parents).
func (s *Store) Mkdir(_ context.Context, rel string) error {
	full, err := resolve(s.notesDir, rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(full, 0o755)
}

// List returns a flat, content-free listing of folders and notes.
func (s *Store) List(_ context.Context) ([]core.Entry, error) {
	var entries []core.Entry
	err := walkEntries(s.notesDir, "", func(rel string, info os.FileInfo, isDir bool) {
		entries = append(entries, core.Entry{
			Path:    rel,
			Name:    filepath.Base(rel),
			IsDir:   isDir,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
		})
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// ListUnder returns notes at or under the given path prefix.
func (s *Store) ListUnder(ctx context.Context, prefix string) ([]core.Note, error) {
	entries, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	prefix = strings.Trim(prefix, "/")
	out := make([]core.Note, 0, 16)
	for _, e := range entries {
		if e.IsDir || !isMarkdown(e.Path) {
			continue
		}
		if prefix != "" && e.Path != prefix && !strings.HasPrefix(e.Path, prefix+"/") {
			continue
		}
		note, err := s.Read(ctx, e.Path)
		if err != nil {
			continue
		}
		out = append(out, note)
	}
	return out, nil
}

// Walk visits every note (reading full content), used to rebuild the index.
func (s *Store) Walk(ctx context.Context, fn func(core.Note) error) error {
	entries, err := s.List(ctx)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir || !isMarkdown(e.Path) {
			continue
		}
		note, err := s.Read(ctx, e.Path)
		if err != nil {
			continue
		}
		if err := fn(note); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// AssetStore
// ---------------------------------------------------------------------------

// Save stores r under name and returns the resulting asset.
func (s *Store) Save(_ context.Context, name string, r io.Reader) (core.Asset, error) {
	ext := filepath.Ext(name)
	base := sanitize(strings.TrimSuffix(filepath.Base(name), ext))
	if base == "" {
		base = "asset"
	}
	id := ulid.Make().String()
	rel := filepath.ToSlash(filepath.Join(time.Now().UTC().Format("2006/01"), id+"-"+base+ext))
	full, err := resolve(s.assetsDir, rel)
	if err != nil {
		return core.Asset{}, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return core.Asset{}, err
	}

	tmp, err := os.CreateTemp(filepath.Dir(full), ".overview-tmp-*")
	if err != nil {
		return core.Asset{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	size, err := io.Copy(tmp, r)
	if err != nil {
		tmp.Close()
		return core.Asset{}, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return core.Asset{}, err
	}
	if err := tmp.Close(); err != nil {
		return core.Asset{}, err
	}
	if err := os.Rename(tmpName, full); err != nil {
		return core.Asset{}, err
	}
	return core.Asset{
		Path:        rel,
		URL:         "/assets/" + rel,
		Name:        filepath.Base(name),
		Size:        size,
		ContentType: contentType(full),
	}, nil
}

// Open returns a reader for the asset at rel.
func (s *Store) Open(_ context.Context, rel string) (io.ReadSeekCloser, error) {
	full, err := resolve(s.assetsDir, rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func buildNote(rel string, raw []byte, info os.FileInfo) core.Note {
	doc := markdown.Parse(string(raw))
	return core.Note{
		ID:      doc.Meta.ID,
		Path:    rel,
		Title:   doc.Meta.Title,
		Tags:    doc.Meta.Tags,
		Icon:    doc.Meta.Icon,
		Created: parseTime(doc.Meta.Created),
		Updated: info.ModTime().UTC(),
		Size:    info.Size(),
		Version: hashBytes(raw),
		Public:  doc.Meta.Public,
		Body:    doc.Body,
	}
}

func checkVersion(expected string, exists bool, current string) error {
	switch expected {
	case "":
		return nil
	case "*":
		if exists {
			return core.Conflictf("note already exists")
		}
		return nil
	default:
		if !exists {
			return core.Conflictf("note no longer exists")
		}
		if expected != current {
			return core.Conflictf("note was modified by another client")
		}
		return nil
	}
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".overview-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

// walkEntries visits every non-hidden entry under root.
func walkEntries(root, rel string, visit func(rel string, info os.FileInfo, isDir bool)) error {
	dir := root
	if rel != "" {
		dir = filepath.Join(root, filepath.FromSlash(rel))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.IsDir() {
			visit(childRel, info, true)
			if err := walkEntries(root, childRel, visit); err != nil {
				return err
			}
			continue
		}
		if !isMarkdown(name) {
			continue
		}
		visit(childRel, info, false)
	}
	return nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func deriveTitle(name, body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func isMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

func toSlashPath(rel string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
}

func contentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
