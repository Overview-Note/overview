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

	for _, f := range []string{"index.html", "world.html", "style.css", "content.css", "search.js", "search-index.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing output %s: %v", f, err)
		}
	}

	// The shared content stylesheet is referenced and the article uses the
	// shared reading class so static pages match the app's reading state.
	css, _ := os.ReadFile(filepath.Join(out, "content.css"))
	if !strings.Contains(string(css), "--hl-keyword") {
		t.Errorf("content.css missing shared syntax tokens")
	}

	// The "Home" note becomes index.html and wiki-links resolve to world.html.
	home, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !strings.Contains(string(home), "world.html") {
		t.Errorf("home page missing wiki-link target: %s", home)
	}
	if !strings.Contains(string(home), `class="doc tiptap-content"`) {
		t.Errorf("page missing shared article class: %s", home)
	}
	if !strings.Contains(string(home), "content.css") {
		t.Errorf("page does not link content.css: %s", home)
	}

	idx, _ := os.ReadFile(filepath.Join(out, "search-index.json"))
	if !strings.Contains(string(idx), "World") {
		t.Errorf("search index missing page: %s", idx)
	}
}

func TestGenerateTheme(t *testing.T) {
	notes := t.TempDir()
	writeNote(t, notes, "Home.md", "# Home\n\nhi")

	for _, tc := range []struct {
		theme string
		want  string
	}{
		{"", `data-theme="auto"`},
		{"auto", `data-theme="auto"`},
		{"light", `data-theme="light"`},
		{"DARK", `data-theme="dark"`},
		{"nonsense", `data-theme="auto"`},
	} {
		out := t.TempDir()
		if _, err := sitegen.Generate(sitegen.Options{
			NotesDir: notes, OutDir: out, SiteTitle: "T", Theme: tc.theme,
		}); err != nil {
			t.Fatalf("theme %q: %v", tc.theme, err)
		}
		home, _ := os.ReadFile(filepath.Join(out, "index.html"))
		if !strings.Contains(string(home), tc.want) {
			t.Errorf("theme %q: index.html missing %s", tc.theme, tc.want)
		}
		css, _ := os.ReadFile(filepath.Join(out, "style.css"))
		if !strings.Contains(string(css), `:root[data-theme="dark"]`) {
			t.Errorf("theme %q: style.css missing dark selector", tc.theme)
		}
		if !strings.Contains(string(css), `prefers-color-scheme:dark`) {
			t.Errorf("theme %q: style.css missing prefers-color-scheme", tc.theme)
		}
	}
}

// featureBody is a fixture exercising every construct the app's pipeline
// special-cases.
var featureBody = strings.Join([]string{
	"# Home",
	"",
	"## Deep Dive",
	"",
	"### Repeat",
	"",
	"### Repeat",
	"",
	"- [x] done",
	"- [ ] todo",
	"",
	"```go",
	"func main() {}",
	"```",
	"",
	"```mermaid",
	"graph TD",
	"  A --> B",
	"```",
	"",
	"Inline $x^2$ math.",
	"",
	"$$",
	"\\int_0^1 f(x)\\,dx",
	"$$",
	"",
	"| a | b |",
	"| - | - |",
	"| 1 | 2 |",
	"",
	"![Pic](assets/2026/10/pic.png)",
	"",
	"Ref[^n].",
	"",
	"[^n]: note body",
	"",
	"See [[World]] and [[Nope]].",
}, "\n")

func TestGenerateFeatureDOM(t *testing.T) {
	notes := t.TempDir()
	assets := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assets, "2026", "10"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "2026", "10", "pic.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNote(t, notes, "Home.md", featureBody)
	writeNote(t, notes, "World.md", "# World\n\nhi")
	out := t.TempDir()

	if _, err := sitegen.Generate(sitegen.Options{
		NotesDir:  notes,
		OutDir:    out,
		Base:      "/docs/",
		SiteTitle: "T",
		AssetsDir: assets,
	}); err != nil {
		t.Fatal(err)
	}

	home, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(home)
	for _, want := range []string{
		`class="code-block"`,
		`class="language-go"`,
		`data-type="taskList"`,
		`data-type="taskItem"`,
		`data-checked="true"`,
		`data-checked="false"`,
		`<label><input type="checkbox" checked="checked" disabled=""`,
		`<div><p>done</p></div>`,
		`class="fn-ref" data-id="n"`,
		`class="fn-defs"`,
		`class="fn-def" data-id="n"`,
		`class="math-inline" data-tex="x^2"`,
		`class="math-block" data-tex="\int_0^1 f(x)\,dx"`,
		`class="mermaid" data-source="graph TD`,
		`class="wiki-link" href="/docs/world.html"`,
		`class="wiki-link wiki-missing"`,
		`<th><p>a</p></th>`,
		`<td><p>1</p></td>`,
		`src="/docs/assets/2026/10/pic.png"`,
		`loading="lazy"`,
		`decoding="async"`,
		`id="deep-dive"`,
		`id="repeat"`,
		`id="repeat-2"`,
		`vendor/highlight/highlight.min.js`,
		`vendor/katex/katex.min.js`,
		`vendor/mermaid/mermaid.min.js`,
		`readonly-enhance.js`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html missing %q", want)
		}
	}

	// Assets are copied into the export.
	if _, err := os.Stat(filepath.Join(out, "assets", "2026", "10", "pic.png")); err != nil {
		t.Errorf("asset not copied: %v", err)
	}
	// Vendored libraries follow the features actually used.
	for _, f := range []string{
		"readonly-enhance.js",
		"vendor/highlight/highlight.min.js",
		"vendor/katex/katex.min.js",
		"vendor/katex/fonts/KaTeX_Main-Regular.woff2",
		"vendor/mermaid/mermaid.min.js",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing vendor output %s: %v", f, err)
		}
	}

	idx, _ := os.ReadFile(filepath.Join(out, "search-index.json"))
	for _, want := range []string{"func main() {}", "x^2", "graph TD", "note body"} {
		if !strings.Contains(string(idx), want) {
			t.Errorf("search index missing source %q: %s", want, idx)
		}
	}
}

func TestGenerateNoVendorWhenUnused(t *testing.T) {
	notes := t.TempDir()
	writeNote(t, notes, "Home.md", "# Home\n\nplain text only")
	out := t.TempDir()
	if _, err := sitegen.Generate(sitegen.Options{NotesDir: notes, OutDir: out, SiteTitle: "T"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "vendor", "highlight")); !os.IsNotExist(err) {
		t.Errorf("highlight vendored for a plain page")
	}
	if _, err := os.Stat(filepath.Join(out, "vendor", "mermaid")); !os.IsNotExist(err) {
		t.Errorf("mermaid vendored for a plain page")
	}
	if _, err := os.Stat(filepath.Join(out, "readonly-enhance.js")); err != nil {
		t.Errorf("readonly-enhance.js missing: %v", err)
	}
}

func TestGenerateAssetURLPrefix(t *testing.T) {
	notes := t.TempDir()
	writeNote(t, notes, "Home.md", "# Home\n\n![Pic](assets/pic.png)")
	out := t.TempDir()
	if _, err := sitegen.Generate(sitegen.Options{
		NotesDir:       notes,
		OutDir:         out,
		SiteTitle:      "T",
		AssetURLPrefix: "https://cdn.example.com/bucket",
	}); err != nil {
		t.Fatal(err)
	}
	home, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !strings.Contains(string(home), `src="https://cdn.example.com/bucket/pic.png"`) {
		t.Errorf("asset not resolved against CDN prefix: %s", home)
	}
}

func TestSlugMatchesDocPipeline(t *testing.T) {
	notes := t.TempDir()
	writeNote(t, notes, "Home.md", "# 标题 Home!\n\n## Hello, World!\n\n## 中文 标题\n\n## A & B < C")
	out := t.TempDir()
	if _, err := sitegen.Generate(sitegen.Options{NotesDir: notes, OutDir: out, SiteTitle: "T"}); err != nil {
		t.Fatal(err)
	}
	home, _ := os.ReadFile(filepath.Join(out, "index.html"))
	for _, want := range []string{
		`id="标题-home"`,
		`id="hello-world"`,
		`id="中文-标题"`,
		// doc.ts slugs the raw rendered HTML (entities left encoded), so `&`
		// becomes the "amp" segment rather than being dropped.
		`id="a-amp-b-lt-c"`,
	} {
		if !strings.Contains(string(home), want) {
			t.Errorf("missing heading anchor %s: %s", want, home)
		}
	}
}
