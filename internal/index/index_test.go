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

func TestManifestIncludesVersion(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	now := time.Now().UTC()
	if err := ix.Sync(ctx, []core.Note{
		{ID: "1", Path: "a.md", Title: "A", Body: "alpha", Version: "v-a", Created: now, Updated: now, Public: true},
		{ID: "2", Path: "dir/b.md", Title: "B", Body: "beta", Version: "v-b", Created: now, Updated: now},
	}); err != nil {
		t.Fatal(err)
	}

	notes, maxSeq, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("manifest notes = %d, want 2", len(notes))
	}
	if notes[0].Path != "a.md" || notes[0].Version != "v-a" || !notes[0].Public {
		t.Errorf("unexpected manifest note: %+v", notes[0])
	}
	if notes[1].Version != "v-b" {
		t.Errorf("version missing: %+v", notes[1])
	}
	if maxSeq <= 0 {
		t.Errorf("maxSeq = %d, want > 0", maxSeq)
	}
}

func TestChangedSeqIncreases(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)

	if err := ix.Upsert(ctx, note("1", "a.md", "A", "alpha")); err != nil {
		t.Fatal(err)
	}
	_, s1, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := ix.Upsert(ctx, note("2", "b.md", "B", "beta")); err != nil {
		t.Fatal(err)
	}
	_, s2, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s2 <= s1 {
		t.Errorf("changed_seq did not increase on upsert: %d -> %d", s1, s2)
	}

	// A subtree replacement treats reinserted notes as changed.
	if err := ix.ReplacePrefix(ctx, "b.md", []core.Note{note("2", "b.md", "B2", "beta2")}); err != nil {
		t.Fatal(err)
	}
	_, s3, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s3 <= s2 {
		t.Errorf("changed_seq did not increase on replace: %d -> %d", s2, s3)
	}
}

func TestSyncPreservesUnchangedSeq(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	notes := []core.Note{{ID: "1", Path: "a.md", Title: "A", Body: "alpha", Version: "v1"}}
	if err := ix.Sync(ctx, notes); err != nil {
		t.Fatal(err)
	}
	_, before, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.Sync(ctx, notes); err != nil {
		t.Fatal(err)
	}
	_, after, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("unchanged full rebuild bumped changed_seq: %d -> %d", before, after)
	}

	changed := []core.Note{{ID: "1", Path: "a.md", Title: "A2", Body: "alpha2", Version: "v2"}}
	if err := ix.Sync(ctx, changed); err != nil {
		t.Fatal(err)
	}
	_, bumped, err := ix.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bumped <= after {
		t.Errorf("content change did not bump changed_seq: %d -> %d", after, bumped)
	}
}

func TestTombstones(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	now := time.Now().UTC().Truncate(time.Second)

	if err := ix.RecordTombstone(ctx, core.Tombstone{ID: "1", Path: "a.md", DeletedAt: now, Device: "laptop"}); err != nil {
		t.Fatal(err)
	}
	if err := ix.RecordTombstone(ctx, core.Tombstone{ID: "1", Path: "a.md", DeletedAt: now.Add(time.Minute), Device: "phone"}); err != nil {
		t.Fatal(err)
	}

	got, err := ix.Tombstones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("tombstones = %d, want 1 (upsert by id)", len(got))
	}
	if got[0].Device != "phone" || got[0].Path != "a.md" {
		t.Errorf("tombstone not updated: %+v", got[0])
	}
}

func TestVaultIDPersists(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	ix, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ix.VaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" {
		t.Fatal("vault id is empty")
	}
	again, err := ix.VaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Errorf("vault id changed within a session: %q -> %q", first, again)
	}
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}

	ix2, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix2.Close() })
	reopened, err := ix2.VaultID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reopened != first {
		t.Errorf("vault id not persisted: %q -> %q", first, reopened)
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
