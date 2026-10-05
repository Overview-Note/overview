package index_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/index"
)

func openTestIndex(t *testing.T) *index.Index {
	t.Helper()
	ix, err := index.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix
}

func note(id, path, title, body string) core.Note {
	now := time.Now().UTC()
	return core.Note{ID: id, Path: path, Title: title, Body: body, Created: now, Updated: now, Version: "v"}
}

func TestSyncAndTree(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	err := ix.Sync(ctx, []core.Note{
		note("1", "a.md", "A", "alpha"),
		note("2", "dir/b.md", "B", "beta"),
	})
	if err != nil {
		t.Fatal(err)
	}
	metas, err := ix.Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("tree len = %d, want 2", len(metas))
	}
	if metas[0].Path != "a.md" || metas[1].Path != "dir/b.md" {
		t.Errorf("unexpected order: %+v", metas)
	}
}

func TestSyncAssignsStableIDs(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	notes := []core.Note{
		note("", "a.md", "A", "alpha"),
		note("", "dir/b.md", "B", "beta"),
	}
	if err := ix.Sync(ctx, notes); err != nil {
		t.Fatalf("sync with empty ids: %v", err)
	}
	first := map[string]string{}
	metas, err := ix.Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("tree len = %d, want 2", len(metas))
	}
	for _, m := range metas {
		if m.ID == "" {
			t.Fatalf("note %q indexed with empty id", m.Path)
		}
		first[m.Path] = m.ID
	}

	if err := ix.Sync(ctx, notes); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	metas, err = ix.Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("tree len after re-sync = %d, want 2", len(metas))
	}
	for _, m := range metas {
		if first[m.Path] != m.ID {
			t.Errorf("id for %q changed across reindex: %q -> %q", m.Path, first[m.Path], m.ID)
		}
	}
}

func TestSyncSkipsDuplicateID(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	err := ix.Sync(ctx, []core.Note{
		note("dup", "a.md", "A", "alpha"),
		note("dup", "b.md", "B", "beta"),
		note("ok", "c.md", "C", "gamma"),
	})
	if err != nil {
		t.Fatalf("a single bad note must not fail the whole sync: %v", err)
	}
	metas, err := ix.Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("tree len = %d, want 2 (one duplicate skipped)", len(metas))
	}
}

func TestSearchCJKInfix(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	if err := ix.Sync(ctx, []core.Note{
		note("1", "go.md", "Go 并发模型", "goroutine 与 channel 的用法"),
	}); err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{"并发", "发模", "模型", "channel", "gorou"} {
		hits, err := ix.Search(ctx, q, 10, 0)
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if len(hits) != 1 {
			t.Errorf("search %q returned %d hits, want 1", q, len(hits))
		}
	}
}

func TestSearchSnippetEscaped(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	if err := ix.Sync(ctx, []core.Note{
		note("1", "x.md", "X", "<script>alert(1)</script> channel"),
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := ix.Search(ctx, "channel", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d", len(hits))
	}
	if strings.Contains(hits[0].Snippet, "<script>") {
		t.Errorf("snippet not escaped: %q", hits[0].Snippet)
	}
	if !strings.Contains(hits[0].Snippet, "<mark>channel</mark>") {
		t.Errorf("snippet missing highlight: %q", hits[0].Snippet)
	}
}

func TestWikiLinks(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	if err := ix.Sync(ctx, []core.Note{
		note("1", "a.md", "A", "see [[Bee]] and [[dir/b]] and [[b]]"),
		note("2", "dir/b.md", "Bee", "hello"),
		note("3", "orphan.md", "Orphan", "nothing"),
	}); err != nil {
		t.Fatal(err)
	}

	out, err := ix.OutgoingLinks(ctx, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("outgoing = %d, want 3", len(out))
	}
	for i, link := range out {
		if link.Target == nil || link.Target.Path != "dir/b.md" {
			t.Errorf("link %d not resolved: %+v", i, link)
		}
	}

	back, err := ix.Backlinks(ctx, "dir/b.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].Path != "a.md" {
		t.Errorf("backlinks = %+v, want [a.md]", back)
	}

	meta, err := ix.ResolveLink(ctx, "Bee")
	if err != nil || meta.Path != "dir/b.md" {
		t.Errorf("resolve by title failed: %+v, %v", meta, err)
	}
	if _, err := ix.ResolveLink(ctx, "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing target should be ErrNotFound, got %v", err)
	}
}

func TestReplacePrefix(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	if err := ix.Sync(ctx, []core.Note{
		note("1", "dir/a.md", "A", "alpha"),
		note("2", "dir/sub/b.md", "B", "beta"),
		note("3", "other.md", "O", "omega"),
	}); err != nil {
		t.Fatal(err)
	}

	// Replace only dir/* with a single new note.
	if err := ix.ReplacePrefix(ctx, "dir", []core.Note{
		note("9", "dir/new.md", "New", "gamma"),
	}); err != nil {
		t.Fatal(err)
	}

	hits, _ := ix.Search(ctx, "alpha", 10, 0)
	if len(hits) != 0 {
		t.Errorf("alpha should be gone: %d", len(hits))
	}
	hits, _ = ix.Search(ctx, "gamma", 10, 0)
	if len(hits) != 1 {
		t.Errorf("gamma should exist: %d", len(hits))
	}
	// Unrelated subtree survives.
	hits, _ = ix.Search(ctx, "omega", 10, 0)
	if len(hits) != 1 {
		t.Errorf("unrelated note lost: %d", len(hits))
	}
}

func TestUpsertAndDelete(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	if err := ix.Upsert(ctx, note("1", "a.md", "A", "hello world")); err != nil {
		t.Fatal(err)
	}
	if err := ix.Upsert(ctx, note("1", "a.md", "A2", "hello world updated")); err != nil {
		t.Fatal(err)
	}
	metas, _ := ix.Tree(ctx)
	if len(metas) != 1 || metas[0].Title != "A2" {
		t.Errorf("upsert failed: %+v", metas)
	}
	if err := ix.DeleteByPath(ctx, "a.md"); err != nil {
		t.Fatal(err)
	}
	metas, _ = ix.Tree(ctx)
	if len(metas) != 0 {
		t.Errorf("delete failed: %+v", metas)
	}
}
