package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/overview-app/overview/internal/core"
	"github.com/overview-app/overview/internal/index"
	"github.com/overview-app/overview/internal/service"
	"github.com/overview-app/overview/internal/store"
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
	return service.New(st, ix, st)
}

func TestTreeAssembly(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	if err := svc.Mkdir(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "root.md", "# Root", "*"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "dir/b.md", "# Bee", "*"); err != nil {
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
	if _, err := svc.SaveNote(ctx, "go.md", "# Go\n\ngoroutine 与 channel", "*"); err != nil {
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
	if _, err := svc.SaveNote(ctx, "dir/a.md", "# A\n\nalpha", "*"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "dir/b.md", "# B\n\nbeta", "*"); err != nil {
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
	first, err := svc.SaveNote(ctx, "n.md", "v1", "*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, "n.md", "v2", "stale"); err == nil {
		t.Error("expected conflict")
	}
	if _, err := svc.SaveNote(ctx, "n.md", "v2", first.Version); err != nil {
		t.Errorf("valid update failed: %v", err)
	}
}
