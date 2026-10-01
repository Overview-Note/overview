package markdown

import (
	"strings"
	"testing"
)

func TestParseWithoutFrontmatter(t *testing.T) {
	doc := Parse("# Hello\n\nbody")
	if doc.Meta.Title != "" {
		t.Errorf("expected empty meta, got %+v", doc.Meta)
	}
	if doc.Body != "# Hello\n\nbody" {
		t.Errorf("unexpected body %q", doc.Body)
	}
}

func TestParseWithFrontmatter(t *testing.T) {
	raw := "---\nid: abc\ntitle: 标题\ntags:\n  - go\n---\n\n# 标题\n\n正文\n"
	doc := Parse(raw)
	if doc.Meta.ID != "abc" || doc.Meta.Title != "标题" {
		t.Errorf("unexpected meta %+v", doc.Meta)
	}
	if len(doc.Meta.Tags) != 1 || doc.Meta.Tags[0] != "go" {
		t.Errorf("unexpected tags %+v", doc.Meta.Tags)
	}
	if doc.Body != "# 标题\n\n正文\n" {
		t.Errorf("unexpected body %q", doc.Body)
	}
}

func TestStringRoundTrip(t *testing.T) {
	orig := Document{
		Meta: Frontmatter{ID: "id1", Title: "标题", Tags: []string{"a", "b"}},
		Body: "# 标题\n\n正文",
	}
	got := Parse(orig.String())
	if got.Meta.ID != "id1" || got.Meta.Title != "标题" {
		t.Errorf("meta mismatch: %+v", got.Meta)
	}
	if got.Body != orig.Body {
		t.Errorf("body mismatch: %q vs %q", got.Body, orig.Body)
	}
}

func TestParseCRLF(t *testing.T) {
	raw := "---\r\nid: x\r\ntitle: t\r\n---\r\n\r\nbody"
	doc := Parse(raw)
	if doc.Meta.ID != "x" {
		t.Errorf("failed to parse CRLF frontmatter: %+v", doc.Meta)
	}
	if !strings.HasPrefix(doc.Body, "body") {
		t.Errorf("unexpected body %q", doc.Body)
	}
}
