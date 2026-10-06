package index_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/index"
)

func TestChangesSinceFiltersAndPaginates(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)

	for i, p := range []string{"a.md", "b.md", "c.md"} {
		if err := ix.Upsert(ctx, note(strconv.Itoa(i), p, p, "body")); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := ix.LatestSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest != 3 {
		t.Fatalf("LatestSeq = %d, want 3", latest)
	}

	all, hasMore, err := ix.ChangesSince(ctx, 0, index.DefaultChangeLimit)
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Errorf("hasMore = true for a complete page")
	}
	if len(all) != 3 {
		t.Fatalf("changes = %d, want 3", len(all))
	}
	if all[0].Seq >= all[1].Seq || all[1].Seq >= all[2].Seq {
		t.Errorf("changes not ordered by seq: %+v", all)
	}
	if all[0].Op != "upsert" || all[0].Version == "" {
		t.Errorf("change missing op or version: %+v", all[0])
	}

	// Pagination: two per page.
	page, more, err := ix.ChangesSince(ctx, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || !more {
		t.Fatalf("page = %d hasMore=%v, want 2/true", len(page), more)
	}
	next, more, err := ix.ChangesSince(ctx, page[1].Seq, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || more {
		t.Fatalf("next page = %d hasMore=%v, want 1/false", len(next), more)
	}

	// A later write appears only above the previous head.
	if err := ix.Upsert(ctx, note("9", "d.md", "d", "body")); err != nil {
		t.Fatal(err)
	}
	inc, _, err := ix.ChangesSince(ctx, latest, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(inc) != 1 || inc[0].Path != "d.md" {
		t.Fatalf("incremental changes = %+v, want [d.md]", inc)
	}

	if got, _, err := ix.ChangesSince(ctx, inc[0].Seq, 10); err != nil {
		t.Fatal(err)
	} else if len(got) != 0 {
		t.Errorf("changes after head = %d, want 0", len(got))
	}
}

func TestFolderTombstones(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	t0 := time.Now().UTC().Truncate(time.Second)

	if err := ix.RecordFolderTombstone(ctx, core.FolderTombstone{Path: "a", DeletedAt: t0, Device: "laptop"}); err != nil {
		t.Fatal(err)
	}
	// A second record for the same path upserts the marker.
	if err := ix.RecordFolderTombstone(ctx, core.FolderTombstone{Path: "a", DeletedAt: t0.Add(time.Minute), Device: "phone"}); err != nil {
		t.Fatal(err)
	}
	if err := ix.RecordFolderTombstone(ctx, core.FolderTombstone{Path: "b/c", DeletedAt: t0.Add(2 * time.Minute)}); err != nil {
		t.Fatal(err)
	}

	all, err := ix.FolderTombstonesSince(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("folder tombstones = %d, want 2 (upsert by path)", len(all))
	}
	if all[0].Path != "a" || all[0].Device != "phone" {
		t.Errorf("folder tombstone not updated: %+v", all[0])
	}

	later, err := ix.FolderTombstonesSince(ctx, t0.Add(90*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(later) != 1 || later[0].Path != "b/c" {
		t.Fatalf("filtered folder tombstones = %+v, want [b/c]", later)
	}

	latest, err := ix.LatestTombstoneTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !latest.Equal(t0.Add(2 * time.Minute)) {
		t.Errorf("LatestTombstoneTime = %v, want %v", latest, t0.Add(2*time.Minute))
	}
}

func TestLatestTombstoneTimeIncludesNotes(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)
	if got, err := ix.LatestTombstoneTime(ctx); err != nil {
		t.Fatal(err)
	} else if !got.IsZero() {
		t.Errorf("empty index LatestTombstoneTime = %v, want zero", got)
	}

	t0 := time.Now().UTC().Truncate(time.Second)
	if err := ix.RecordTombstone(ctx, core.Tombstone{ID: "1", Path: "a.md", DeletedAt: t0}); err != nil {
		t.Fatal(err)
	}
	if err := ix.RecordFolderTombstone(ctx, core.FolderTombstone{Path: "a", DeletedAt: t0.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	got, err := ix.LatestTombstoneTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(t0.Add(time.Minute)) {
		t.Errorf("LatestTombstoneTime = %v, want %v", got, t0.Add(time.Minute))
	}
}
