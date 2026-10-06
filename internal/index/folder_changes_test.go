package index_test

import (
	"context"
	"testing"
)

// TestFolderChangeAdvancesLatestSeq verifies that recording an empty folder
// advances the shared change sequence (without touching the tombstone cursor)
// so a sync client observes the folder even though no note changed.
func TestFolderChangeAdvancesLatestSeq(t *testing.T) {
	ctx := context.Background()
	ix := openTestIndex(t)

	base, err := ix.LatestSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.RecordFolderChange(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
	afterFolder, err := ix.LatestSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterFolder <= base {
		t.Fatalf("LatestSeq after folder change = %d, want > %d", afterFolder, base)
	}

	if err := ix.Upsert(ctx, note("1", "a.md", "a", "body")); err != nil {
		t.Fatal(err)
	}
	afterNote, err := ix.LatestSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterNote <= afterFolder {
		t.Fatalf("LatestSeq after note write = %d, want > %d", afterNote, afterFolder)
	}

	// Re-recording the same folder allocates a fresh, larger sequence.
	if err := ix.RecordFolderChange(ctx, "empty"); err != nil {
		t.Fatal(err)
	}
	again, err := ix.LatestSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again <= afterNote {
		t.Fatalf("LatestSeq after second folder change = %d, want > %d", again, afterNote)
	}
}
