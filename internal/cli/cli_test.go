package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return stdout.String(), stderr.String(), code
}

func TestWriteReadSearch(t *testing.T) {
	dir := t.TempDir()

	out, errOut, code := run(t, "-C", dir, "write", "Guide/Intro.md",
		"--body", "# Intro\n\nhello world [[Setup]]", "--public")
	if code != 0 {
		t.Fatalf("write code = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "Guide/Intro.md") {
		t.Fatalf("write output = %q", out)
	}

	out, _, code = run(t, "-C", dir, "read", "Guide/Intro.md")
	if code != 0 || !strings.Contains(out, "hello world") {
		t.Fatalf("read code = %d, out = %q", code, out)
	}

	out, _, code = run(t, "-C", dir, "search", "hello", "--json")
	if code != 0 {
		t.Fatalf("search code = %d", code)
	}
	var hits []map[string]any
	if err := json.Unmarshal([]byte(out), &hits); err != nil {
		t.Fatalf("search json: %v (%s)", err, out)
	}
	if len(hits) != 1 || hits[0]["path"] != "Guide/Intro.md" {
		t.Fatalf("search hits = %v", hits)
	}
}

func TestFlagsAfterPositional(t *testing.T) {
	dir := t.TempDir()
	if _, errOut, code := run(t, "-C", dir, "write", "a.md", "--body", "content-here"); code != 0 {
		t.Fatalf("write: %s", errOut)
	}
	out, _, _ := run(t, "-C", dir, "read", "a.md")
	if !strings.Contains(out, "content-here") {
		t.Fatalf("flags after positional were ignored: %q", out)
	}
}

func TestListJSONAndMove(t *testing.T) {
	dir := t.TempDir()
	run(t, "-C", dir, "write", "note.md", "--body", "# Note")
	run(t, "-C", dir, "mkdir", "docs")
	if _, errOut, code := run(t, "-C", dir, "move", "note.md", "docs/note.md"); code != 0 {
		t.Fatalf("move: %s", errOut)
	}

	out, _, code := run(t, "-C", dir, "list", "--json")
	if code != 0 {
		t.Fatalf("list code = %d", code)
	}
	if !strings.Contains(out, `"docs/note.md"`) {
		t.Fatalf("moved note missing from tree: %s", out)
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "vault.zip")

	run(t, "-C", src, "write", "a.md", "--body", "# A\n\nalpha")
	run(t, "-C", src, "write", "b/c.md", "--body", "# C\n\ngamma")

	if _, errOut, code := run(t, "-C", src, "export-zip", zipPath); code != 0 {
		t.Fatalf("export-zip: %s", errOut)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("archive not written: %v", err)
	}

	if _, errOut, code := run(t, "-C", dst, "import", zipPath); code != 0 {
		t.Fatalf("import: %s", errOut)
	}
	out, _, _ := run(t, "-C", dst, "read", "b/c.md")
	if !strings.Contains(out, "gamma") {
		t.Fatalf("imported note missing: %q", out)
	}
}

func TestHistoryAndRestore(t *testing.T) {
	dir := t.TempDir()
	run(t, "-C", dir, "write", "n.md", "--body", "v1")
	run(t, "-C", dir, "write", "n.md", "--body", "v2")

	out, _, code := run(t, "-C", dir, "history", "n.md", "--json")
	if code != 0 {
		t.Fatalf("history code = %d", code)
	}
	var revs []map[string]any
	if err := json.Unmarshal([]byte(out), &revs); err != nil || len(revs) == 0 {
		t.Fatalf("history json = %v (%s)", err, out)
	}
	id, _ := revs[0]["id"].(string)
	if _, errOut, code := run(t, "-C", dir, "restore", "n.md", id); code != 0 {
		t.Fatalf("restore: %s", errOut)
	}
	body, _, _ := run(t, "-C", dir, "read", "n.md")
	if !strings.Contains(body, "v1") {
		t.Fatalf("restore did not apply: %q", body)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, errOut, code := run(t, "nope")
	if code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
}
