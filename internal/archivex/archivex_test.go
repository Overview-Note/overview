package archivex_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Overview-Note/overview/internal/archivex"
	"github.com/Overview-Note/overview/internal/store"
)

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := store.New(t.TempDir(), t.TempDir())

	if _, err := src.Write(ctx, "guide/a.md", "# A\n\nhello", "*", true); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Save(ctx, "pic.png", bytes.NewReader([]byte("PNGDATA"))); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := archivex.NewExporter(src, src).Export(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty archive")
	}

	dstNotes := t.TempDir()
	dst := store.New(dstNotes, t.TempDir())
	res, err := archivex.NewImporter(dst, dst).Import(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if res.Notes != 1 || res.Assets != 1 {
		t.Fatalf("got notes=%d assets=%d, want notes=1 assets=1", res.Notes, res.Assets)
	}

	note, err := dst.Read(ctx, "guide/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.Body != "# A\n\nhello" {
		t.Fatalf("body = %q", note.Body)
	}
	if _, err := os.Stat(filepath.Join(dstNotes, "guide", "a.md")); err != nil {
		t.Fatalf("restored note file missing: %v", err)
	}
	assets, err := dst.ListAssets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets = %d, want 1", len(assets))
	}
}
