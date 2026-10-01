package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/overview-app/overview/internal/core"
	"github.com/overview-app/overview/internal/history"
	"github.com/overview-app/overview/internal/index"
	"github.com/overview-app/overview/internal/service"
	"github.com/overview-app/overview/internal/store"
	"github.com/overview-app/overview/internal/trash"
)

func newService(t *testing.T) *service.Service {
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
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	return service.New(st, ix, st, nil, nil)
}

func newServiceWithLifecycle(t *testing.T) *service.Service {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	_ = os.MkdirAll(notes, 0o755)
	_ = os.MkdirAll(assets, 0o755)
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	hist := history.New(filepath.Join(root, ".history"))
	tr := trash.New(filepath.Join(root, ".trash"))
	return service.New(st, ix, st, hist, tr)
}

func TestHistoryAndTrash(t *testing.T) {
	ctx := context.Background()
	svc := newServiceWithLifecycle(t)

	first, err := svc.SaveNote(ctx, "doc.md", "# v1\n\nfirst", "*", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "doc.md", "# v2\n\nsecond", first.Version, false); err != nil {
		t.Fatal(err)
	}

	revs, err := svc.Revisions(ctx, "doc.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) == 0 {
		t.Fatal("expected at least one revision")
	}

	content, err := svc.RevisionContent(ctx, "doc.md", revs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "first") {
		t.Errorf("revision should hold v1 body: %q", content)
	}

	restored, err := svc.RestoreRevision(ctx, "doc.md", revs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.Body, "first") {
		t.Errorf("restore failed: %q", restored.Body)
	}

	// Soft delete then restore from trash.
	if err := svc.Delete(ctx, "doc.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetNote(ctx, "doc.md"); err == nil {
		t.Error("note should be gone after delete")
	}
	entries, err := svc.Trash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("trash entries = %d, want 1", len(entries))
	}
	if err := svc.RestoreTrash(ctx, entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetNote(ctx, "doc.md"); err != nil {
		t.Errorf("note should be restored: %v", err)
	}
	if err := svc.PurgeTrash(ctx, entries[0].ID); err != nil {
		t.Errorf("purge: %v", err)
	}
}

func TestTreeAssembly(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	if err := svc.Mkdir(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "root.md", "# Root", "*", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "dir/b.md", "# Bee", "*", false); err != nil {
		t.Fatal(err)
	}

	nodes, err := svc.Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("root nodes = %d, want 3: %+v", len(nodes), nodes)
	}
	// folders first, then notes
	if nodes[0].Type != core.NodeFolder || nodes[1].Type != core.NodeFolder {
		t.Errorf("folders should sort first: %+v", nodes)
	}
	if nodes[2].Type != core.NodeNote || nodes[2].Title != "Root" {
		t.Errorf("note missing title: %+v", nodes[2])
	}
	var dir *core.TreeNode
	for i := range nodes {
		if nodes[i].Path == "dir" {
			dir = &nodes[i]
		}
	}
	if dir == nil || len(dir.Children) != 1 || dir.Children[0].Title != "Bee" {
		t.Errorf("nested note title not resolved from index: %+v", dir)
	}
}

func TestSaveSearchDelete(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	if _, err := svc.SaveNote(ctx, "go.md", "# Go\n\ngoroutine 与 channel", "*", false); err != nil {
		t.Fatal(err)
	}
	hits, err := svc.Search(ctx, "channel", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if err := svc.Delete(ctx, "go.md"); err != nil {
		t.Fatal(err)
	}
	hits, _ = svc.Search(ctx, "channel", 10, 0)
	if len(hits) != 0 {
		t.Errorf("index not cleaned after delete: %d hits", len(hits))
	}
}

func TestDeleteFolderReindexes(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	if _, err := svc.SaveNote(ctx, "dir/a.md", "# A\n\nalpha", "*", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "dir/b.md", "# B\n\nbeta", "*", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "dir"); err != nil {
		t.Fatal(err)
	}
	hits, _ := svc.Search(ctx, "alpha", 10, 0)
	if len(hits) != 0 {
		t.Errorf("folder delete should reindex, still found %d", len(hits))
	}
}

func TestConflictPropagates(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	first, err := svc.SaveNote(ctx, "n.md", "v1", "*", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "n.md", "v2", "stale", false); err == nil {
		t.Error("expected conflict")
	}
	if _, err := svc.SaveNote(ctx, "n.md", "v2", first.Version, false); err != nil {
		t.Errorf("valid update failed: %v", err)
	}
}
