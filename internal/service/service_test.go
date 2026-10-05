package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
	"github.com/Overview-Note/overview/internal/trash"
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
	hist := history.New(filepath.Join(root, ".history"), 0)
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
	// A restored item must no longer be listed in the trash.
	if rest, _ := svc.Trash(ctx); len(rest) != 0 {
		t.Errorf("trash should be empty after restore, got %d", len(rest))
	}

	// Delete again and purge permanently.
	if err := svc.Delete(ctx, "doc.md"); err != nil {
		t.Fatal(err)
	}
	pending, err := svc.Trash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("trash entries = %d, want 1", len(pending))
	}
	if err := svc.PurgeTrash(ctx, pending[0].ID); err != nil {
		t.Errorf("purge: %v", err)
	}
	if left, _ := svc.Trash(ctx); len(left) != 0 {
		t.Errorf("trash should be empty after purge, got %d", len(left))
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

func TestOrphanAssets(t *testing.T) {
	ctx := context.Background()
	svc := newServiceWithLifecycle(t)

	used, err := svc.Upload(ctx, "used.png", strings.NewReader("USED"))
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := svc.Upload(ctx, "orphan.png", strings.NewReader("ORPHAN"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SaveNote(ctx, "ref.md", "# Ref\n\n![img](assets/"+used.Path+")", "*", false); err != nil {
		t.Fatal(err)
	}

	orphans, err := svc.OrphanAssets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 1 || orphans[0].Path != orphan.Path {
		t.Fatalf("orphans = %+v, want [%s]", orphans, orphan.Path)
	}

	removed, err := svc.PurgeOrphanAssets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if r, err := svc.OpenAsset(ctx, used.Path); err != nil {
		t.Errorf("referenced asset must survive: %v", err)
	} else {
		r.Close()
	}
	if r, err := svc.OpenAsset(ctx, orphan.Path); err == nil {
		r.Close()
		t.Error("orphan asset should be deleted")
	}
}

func allTreeNodes(nodes []core.TreeNode) []core.TreeNode {
	var out []core.TreeNode
	for _, n := range nodes {
		out = append(out, n)
		out = append(out, allTreeNodes(n.Children)...)
	}
	return out
}

func newServiceWithNotes(t *testing.T) (*service.Service, string) {
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
	return service.New(st, ix, st, nil, nil), notes
}

func TestReindexHandlesNotesWithoutFrontmatter(t *testing.T) {
	ctx := context.Background()
	svc, notes := newServiceWithNotes(t)
	if err := os.MkdirAll(filepath.Join(notes, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range map[string]string{
		"plain.md":     "# Plain\n\nalpha body",
		"dir/other.md": "no frontmatter here beta",
	} {
		if err := os.WriteFile(filepath.Join(notes, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ids := map[string]string{}
	for pass := 0; pass < 2; pass++ {
		if err := svc.Reindex(ctx); err != nil {
			t.Fatalf("reindex pass %d: %v", pass, err)
		}
		tree, err := svc.Tree(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		for _, n := range allTreeNodes(tree) {
			if n.Type != core.NodeNote {
				continue
			}
			if n.ID == "" {
				t.Errorf("note %q indexed with empty id", n.Path)
			}
			if pass == 0 {
				ids[n.Path] = n.ID
			} else if ids[n.Path] != n.ID {
				t.Errorf("id for %q changed across reindex: %q -> %q", n.Path, ids[n.Path], n.ID)
			}
			count++
		}
		if count != 2 {
			t.Fatalf("pass %d indexed %d notes, want 2", pass, count)
		}
	}
}

func TestReindexSkipsCorruptNote(t *testing.T) {
	ctx := context.Background()
	svc, notes := newServiceWithNotes(t)
	files := map[string]string{
		"a.md": "---\nid: dup\n---\n\nalpha",
		"b.md": "---\nid: dup\n---\n\nbeta",
		"c.md": "gamma",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(notes, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.Reindex(ctx); err != nil {
		t.Fatalf("a corrupt note must not fail the whole reindex: %v", err)
	}
	gamma, err := svc.Search(ctx, "gamma", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gamma) != 1 {
		t.Errorf("healthy note lost from index: %d hits", len(gamma))
	}
	alpha, _ := svc.Search(ctx, "alpha", 10, 0)
	beta, _ := svc.Search(ctx, "beta", 10, 0)
	if len(alpha)+len(beta) != 1 {
		t.Errorf("duplicate id not isolated: alpha=%d beta=%d", len(alpha), len(beta))
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

func TestDeleteAndMoveRecordTombstones(t *testing.T) {
	ctx := context.Background()
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
	tr := trash.New(filepath.Join(root, ".trash"))
	svc := service.New(st, ix, st, nil, tr)

	if _, err := svc.SaveNote(ctx, "a.md", "# A", "*", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "a.md"); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	if _, err := svc.SaveNote(ctx, "dir/b.md", "# B", "*", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "dir"); err != nil {
		t.Fatalf("delete folder: %v", err)
	}

	if _, err := svc.SaveNote(ctx, "old/c.md", "# C", "*", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Move(ctx, "old", "new"); err != nil {
		t.Fatalf("move folder: %v", err)
	}

	tombs, err := ix.Tombstones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, tb := range tombs {
		paths[tb.Path] = true
		if tb.ID == "" {
			t.Errorf("tombstone without id: %+v", tb)
		}
	}
	for _, want := range []string{"a.md", "dir/b.md", "old/c.md"} {
		if !paths[want] {
			t.Errorf("missing tombstone for %s: %+v", want, tombs)
		}
	}
}

func TestManifestFoldersAndRawWrite(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	if err := svc.Mkdir(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
	content := []byte("---\nid: sync-id\ncreated: \"2024-01-01T00:00:00Z\"\n---\n\n# T\n\nbody\n")
	note, err := svc.SaveNoteRaw(ctx, "dir/raw.md", content, "*")
	if err != nil {
		t.Fatalf("raw save: %v", err)
	}
	sum := sha256.Sum256(content)
	if note.Version != hex.EncodeToString(sum[:]) {
		t.Errorf("version = %q, want hash of raw bytes", note.Version)
	}
	if note.ID != "sync-id" {
		t.Errorf("id = %q, want sync-id", note.ID)
	}

	if _, err := svc.SaveNoteRaw(ctx, "dir/raw.md", content, ""); err == nil {
		t.Error("empty baseVersion must be rejected")
	}
	if _, err := svc.SaveNoteRaw(ctx, "dir/raw.md", content, "stale"); err == nil {
		t.Error("stale baseVersion must conflict")
	}

	m, err := svc.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m.VaultID == "" || m.ETag == "" {
		t.Fatalf("manifest missing ids: %+v", m)
	}
	if len(m.Notes) != 1 || m.Notes[0].Version != note.Version {
		t.Errorf("manifest notes = %+v", m.Notes)
	}
	hasEmpty := false
	for _, f := range m.Folders {
		if f == "empty" {
			hasEmpty = true
		}
	}
	if !hasEmpty {
		t.Errorf("empty folder missing from manifest: %v", m.Folders)
	}
}
