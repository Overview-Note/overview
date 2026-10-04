package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
	"github.com/Overview-Note/overview/internal/tools"
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
	hist := history.New(filepath.Join(root, ".history"), 50)
	tr := trash.New(filepath.Join(root, ".trash"))
	return service.New(st, ix, st, hist, tr)
}

func TestDefsAreComplete(t *testing.T) {
	defs := tools.Defs()
	if len(defs) != 22 {
		t.Fatalf("expected 22 tools, got %d", len(defs))
	}
	names := map[string]bool{}
	for _, d := range defs {
		if d.Name == "" || d.Description == "" {
			t.Errorf("incomplete def: %+v", d)
		}
		if d.InputSchema == nil {
			t.Errorf("missing schema for %s", d.Name)
		}
		if d.InputSchema["type"] != "object" {
			t.Errorf("%s schema type = %v", d.Name, d.InputSchema["type"])
		}
		if _, ok := d.InputSchema["properties"]; !ok {
			t.Errorf("%s schema has no properties", d.Name)
		}
		if !tools.Known(d.Name) {
			t.Errorf("%s not reported as known", d.Name)
		}
		if names[d.Name] {
			t.Errorf("duplicate tool %s", d.Name)
		}
		names[d.Name] = true
	}
	// The formerly-empty schemas must now be explicit objects.
	for _, n := range []string{"notes_list", "assets_orphans", "public_notes", "notes_reindex"} {
		for _, d := range defs {
			if d.Name == n {
				if _, ok := d.InputSchema["properties"]; !ok {
					t.Errorf("%s schema not explicit: %v", n, d.InputSchema)
				}
			}
		}
	}
}

func TestDefsOpenAIShape(t *testing.T) {
	all := tools.DefsOpenAI(nil)
	if len(all) != 22 {
		t.Fatalf("expected 22 openai tools, got %d", len(all))
	}
	first, _ := all[0].(map[string]any)
	if first["type"] != "function" {
		t.Errorf("type = %v", first["type"])
	}
	fn, _ := first["function"].(map[string]any)
	if fn["name"] == "" || fn["parameters"] == nil {
		t.Errorf("function shape = %v", fn)
	}

	filtered := tools.DefsOpenAI(func(name string) bool { return name == "notes_read" })
	if len(filtered) != 1 {
		t.Fatalf("filtered = %d, want 1", len(filtered))
	}
}

func TestRiskAndAllows(t *testing.T) {
	cases := map[string]string{
		"notes_read":   tools.RiskRead,
		"notes_search": tools.RiskRead,
		"notes_write":  tools.RiskWrite,
		"notes_move":   tools.RiskWrite,
		"notes_delete": tools.RiskDangerous,
		"trash_purge":  tools.RiskDangerous,
		"assets_purge": tools.RiskDangerous,
		"mystery":      tools.RiskDangerous,
	}
	for name, want := range cases {
		if got := tools.Risk(name); got != want {
			t.Errorf("Risk(%s) = %s, want %s", name, got, want)
		}
	}

	if !tools.Allows("member", "notes_read") {
		t.Error("member should read")
	}
	if !tools.Allows("member", "notes_write") {
		t.Error("member should write")
	}
	if tools.Allows("member", "notes_delete") {
		t.Error("member must not run dangerous tools")
	}
	if !tools.Allows("admin", "notes_delete") {
		t.Error("admin should run dangerous tools")
	}
	if !tools.Allows("owner", "trash_purge") {
		t.Error("owner should run dangerous tools")
	}
	if tools.Allows("", "notes_read") {
		t.Error("unknown role must be denied")
	}
}

func TestPreview(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	set := tools.NewSet(svc)

	note, err := svc.SaveNote(ctx, "dir/demo.md", "hello world", "*", false)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := set.Preview(ctx, "notes_delete", json.RawMessage(`{"path":"dir/demo.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview, note.Title) || !strings.Contains(preview, "dir/demo.md") {
		t.Errorf("delete preview = %q", preview)
	}

	preview, _ = set.Preview(ctx, "notes_move", json.RawMessage(`{"from":"a.md","to":"b.md"}`))
	if !strings.Contains(preview, "a.md") || !strings.Contains(preview, "b.md") {
		t.Errorf("move preview = %q", preview)
	}

	// Read tools have no preview.
	if p, _ := set.Preview(ctx, "notes_read", json.RawMessage(`{"path":"dir/demo.md"}`)); p != "" {
		t.Errorf("read preview should be empty, got %q", p)
	}

	// Purge preview reports the affected count.
	if err := svc.Delete(ctx, "dir/demo.md"); err != nil {
		t.Fatal(err)
	}
	preview, _ = set.Preview(ctx, "trash_purge", json.RawMessage(`{}`))
	if !strings.Contains(preview, "1") {
		t.Errorf("trash purge preview = %q", preview)
	}
}

func TestExecUnknownTool(t *testing.T) {
	svc := newService(t)
	set := tools.NewSet(svc)
	_, err := set.Exec(context.Background(), "nope", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool: nope") {
		t.Fatalf("err = %v", err)
	}
}
