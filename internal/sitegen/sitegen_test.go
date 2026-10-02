package sitegen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/sitegen"
)

func writeNote(t *testing.T, dir, name, body string) {
	t.Helper()
	content := "---\nid: 01" + name + "\ntitle: " + strings.TrimSuffix(name, ".md") +
		"\npublic: true\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerate(t *testing.T) {
	notes := t.TempDir()
	writeNote(t, notes, "Home.md", "# Home\n\nWelcome. See [[World]].")
	writeNote(t, notes, "World.md", "# World\n\nHello world.\n\n## Section\n\ntext")
	out := t.TempDir()

	n, err := sitegen.Generate(sitegen.Options{
		NotesDir:  notes,
		OutDir:    out,
		Base:      "/",
		SiteTitle: "Test Docs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("pages = %d, want 2", n)
	}

	for _, f := range []string{"index.html", "world.html", "style.css", "search.js", "search-index.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing output %s: %v", f, err)
		}
	}

	// The "Home" note becomes index.html and wiki-links resolve to world.html.
	home, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !strings.Contains(string(home), "world.html") {
		t.Errorf("home page missing wiki-link target: %s", home)
	}

	idx, _ := os.ReadFile(filepath.Join(out, "search-index.json"))
	if !strings.Contains(string(idx), "World") {
		t.Errorf("search index missing page: %s", idx)
	}
}
