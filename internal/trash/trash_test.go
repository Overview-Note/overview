package trash_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Overview-Note/overview/internal/trash"
)

func TestMoveListRestorePurge(t *testing.T) {
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(notes, "a.md")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tr := trash.New(filepath.Join(root, ".trash"))
	entry, err := tr.Move(src, "a.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be gone after move")
	}

	list, err := tr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != entry.ID {
		t.Fatalf("list = %+v", list)
	}

	// Restore back to a new location.
	dst := filepath.Join(notes, "restored.md")
	if err := tr.Restore(entry.ID, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "hello" {
		t.Fatalf("restored content = %q err=%v", data, err)
	}

	// Move again and purge.
	if _, err := tr.Move(dst, "restored.md", false); err != nil {
		t.Fatal(err)
	}
	list, _ = tr.List()
	if err := tr.Purge(list[0].ID); err != nil {
		t.Fatal(err)
	}
	list, _ = tr.List()
	if len(list) != 0 {
		t.Fatalf("trash not empty after purge: %+v", list)
	}
}

func TestPurgeMissing(t *testing.T) {
	tr := trash.New(t.TempDir())
	if err := tr.Purge("nope"); err == nil {
		t.Fatal("expected error purging missing entry")
	}
}
