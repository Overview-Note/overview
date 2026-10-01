package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/overview-app/overview/internal/core"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	return New(notes, assets)
}

func TestResolveRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"", "..", "../x", "a/../../b"} {
		if _, err := resolve(root, bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	ok, err := resolve(root, "a/b.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rel, err := filepath.Rel(root, ok)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Errorf("resolved path escapes root: %q", ok)
	}
}

func TestWriteRead(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	note, err := s.Write(ctx, "技术/Go/并发.md", "# 并发\n\n正文", "*")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if note.ID == "" || note.Version == "" {
		t.Fatalf("missing id/version: %+v", note)
	}
	if note.Title != "并发" {
		t.Errorf("title = %q", note.Title)
	}

	got, err := s.Read(ctx, "技术/Go/并发.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Body != "# 并发\n\n正文" {
		t.Errorf("body = %q", got.Body)
	}
	if got.Version != note.Version {
		t.Errorf("version mismatch")
	}
	if _, err := os.Stat(filepath.Join(s.NotesDir(), "技术", "Go", "并发.md")); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestVersionContracts(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	first, err := s.Write(ctx, "n.md", "v1", "*")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Write(ctx, "n.md", "v2", "*"); !errors.Is(err, core.ErrConflict) {
		t.Errorf("create over existing should conflict, got %v", err)
	}
	if _, err := s.Write(ctx, "n.md", "v2", "deadbeef"); !errors.Is(err, core.ErrConflict) {
		t.Errorf("stale version should conflict, got %v", err)
	}
	if _, err := s.Write(ctx, "other.md", "v2", first.Version); !errors.Is(err, core.ErrConflict) {
		t.Errorf("missing note with version should conflict, got %v", err)
	}

	updated, err := s.Write(ctx, "n.md", "v2", first.Version)
	if err != nil {
		t.Fatalf("valid update failed: %v", err)
	}
	if updated.Version == first.Version {
		t.Errorf("version should change after write")
	}
}

func TestListAndWalk(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.Write(ctx, "a.md", "A", "*"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write(ctx, "dir/b.md", "B", "*"); err != nil {
		t.Fatal(err)
	}

	entries, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = e.IsDir
	}
	if _, ok := paths["a.md"]; !ok {
		t.Errorf("missing a.md in %v", entries)
	}
	if isDir, ok := paths["dir"]; !ok || !isDir {
		t.Errorf("missing dir folder in %v", entries)
	}

	var count int
	if err := s.Walk(ctx, func(core.Note) error { count++; return nil }); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("walk count = %d, want 2", count)
	}
}

func TestDeleteAndMove(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.Write(ctx, "x.md", "X", "*"); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(ctx, "x.md", "sub/y.md"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := s.Read(ctx, "x.md"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("old path should be gone, got %v", err)
	}
	if _, err := s.Read(ctx, "sub/y.md"); err != nil {
		t.Errorf("new path should exist, got %v", err)
	}
	if err := s.Delete(ctx, "sub"); err != nil {
		t.Fatalf("delete folder: %v", err)
	}
	if _, err := s.Read(ctx, "sub/y.md"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("note should be deleted, got %v", err)
	}
}

func TestAssetSaveOpen(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	asset, err := s.Save(ctx, "hello.png", bytes.NewBufferString("PNGDATA"))
	if err != nil {
		t.Fatalf("save asset: %v", err)
	}
	if asset.ContentType != "image/png" {
		t.Errorf("content type = %q", asset.ContentType)
	}
	r, err := s.Open(ctx, asset.Path)
	if err != nil {
		t.Fatalf("open asset: %v", err)
	}
	defer r.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if buf.String() != "PNGDATA" {
		t.Errorf("asset content = %q", buf.String())
	}
	if _, err := s.Open(ctx, "../escape"); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("path escape should be invalid, got %v", err)
	}
}
