// Package sitegen exports a Markdown vault to a static, read-only HTML site.
// It powers `overview export`, which builds the project's documentation site
// from Overview notes so it can be hosted on any static host (e.g. GitHub Pages).
package sitegen

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Overview-Note/overview/internal/markdown"
)

// Page is a rendered note.
type Page struct {
	Title    string
	Slug     string // file name without extension, e.g. "getting-started"
	Path     string // vault-relative source path
	HTML     template.HTML
	Headings []Heading

	HasCode    bool
	HasMath    bool
	HasMermaid bool
}

// Heading is an entry in a page's outline.
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Options configures the export.
type Options struct {
	NotesDir       string
	OutDir         string
	Base           string // URL base path, e.g. "/overview/" for GitHub Pages
	SiteTitle      string
	All            bool   // when true, export every note (not just public ones)
	Theme          string // "auto" (follow system), "light" or "dark"; default auto
	AssetsDir      string // local asset root copied to <out>/assets (empty to skip)
	AssetURLPrefix string // absolute asset base (e.g. an S3/CDN URL); when set, AssetsDir is not copied
}

// normalizeTheme maps arbitrary input to one of auto/light/dark.
func normalizeTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return "auto"
	}
}

// contentCSS is the shared reading typography used by both the app and the
// generated site. It lives next to this package so go:embed can pick it up.
//
//go:embed content.css
var contentCSS string

// Generate renders every public note in NotesDir to a static site in OutDir.
func Generate(opts Options) (int, error) {
	base := opts.Base
	if base == "" {
		base = "/"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	if opts.SiteTitle == "" {
		opts.SiteTitle = "Overview"
	}
	theme := normalizeTheme(opts.Theme)

	assetBase := base + "assets/"
	if opts.AssetURLPrefix != "" {
		assetBase = strings.TrimRight(opts.AssetURLPrefix, "/") + "/"
	}

	docs, err := loadNotes(opts.NotesDir, opts.All)
	if err != nil {
		return 0, err
	}

	// Build lookup tables for wiki-link resolution.
	byTitle := map[string]string{}
	byName := map[string]string{}
	for _, d := range docs {
		byTitle[strings.ToLower(d.Meta.Title)] = d.slug
		byName[strings.ToLower(strings.TrimSuffix(filepath.Base(d.path), ".md"))] = d.slug
	}
	resolve := func(target string) (string, bool) {
		key := strings.ToLower(target)
		slug, ok := byTitle[key]
		if !ok {
			slug, ok = byName[key]
		}
		if !ok {
			return "", false
		}
		return base + url.PathEscape(slug) + ".html", true
	}

	pages := make([]Page, 0, len(docs))
	var anyCode, anyMath, anyMermaid bool
	for _, d := range docs {
		rr := renderNote(d.body, resolve, assetBase)
		title := d.Meta.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(d.path), ".md")
		}
		anyCode = anyCode || rr.HasCode
		anyMath = anyMath || rr.HasMath
		anyMermaid = anyMermaid || rr.HasMermaid
		pages = append(pages, Page{
			Title:      title,
			Slug:       d.slug,
			Path:       d.path,
			HTML:       template.HTML(rr.HTML),
			Headings:   rr.Headings,
			HasCode:    rr.HasCode,
			HasMath:    rr.HasMath,
			HasMermaid: rr.HasMermaid,
		})
	}

	sort.Slice(pages, func(i, j int) bool {
		return strings.ToLower(pages[i].Title) < strings.ToLower(pages[j].Title)
	})

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "style.css"), []byte(siteCSS), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "content.css"), []byte(contentCSS), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "search.js"), []byte(siteJS), 0o644); err != nil {
		return 0, err
	}
	if err := writeSearchIndex(opts.OutDir, pages, base); err != nil {
		return 0, err
	}
	if err := writeVendor(opts.OutDir, anyCode, anyMath, anyMermaid); err != nil {
		return 0, err
	}
	if opts.AssetsDir != "" {
		if err := copyDir(opts.AssetsDir, filepath.Join(opts.OutDir, "assets")); err != nil {
			return 0, err
		}
	}

	nav := buildNav(pages, base)
	for _, p := range pages {
		out := filepath.Join(opts.OutDir, p.Slug+".html")
		var buf bytes.Buffer
		if err := pageTmpl.Execute(&buf, map[string]any{
			"SiteTitle":  opts.SiteTitle,
			"Theme":      theme,
			"Title":      p.Title,
			"Base":       base,
			"Nav":        nav,
			"Content":    p.HTML,
			"Headings":   p.Headings,
			"HasCode":    p.HasCode,
			"HasMath":    p.HasMath,
			"HasMermaid": p.HasMermaid,
			"IsHome":     false,
		}); err != nil {
			return 0, err
		}
		if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
			return 0, err
		}
	}

	// A note named "Home" becomes the landing page (index.html); otherwise a
	// generated list of notes is used.
	hasIndex := false
	for _, p := range pages {
		if p.Slug == "index" {
			hasIndex = true
			break
		}
	}
	if !hasIndex {
		var idx bytes.Buffer
		if err := indexTmpl.Execute(&idx, map[string]any{
			"SiteTitle": opts.SiteTitle,
			"Theme":     theme,
			"Base":      base,
			"Nav":       nav,
			"Pages":     pages,
		}); err != nil {
			return 0, err
		}
		if err := os.WriteFile(filepath.Join(opts.OutDir, "index.html"), idx.Bytes(), 0o644); err != nil {
			return 0, err
		}
	}

	return len(pages), nil
}

type doc struct {
	path string
	slug string
	body string
	Meta markdown.Frontmatter
}

func loadNotes(notesDir string, all bool) ([]doc, error) {
	var docs []doc
	err := filepath.WalkDir(notesDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		parsed := markdown.Parse(string(raw))
		if !all && !parsed.Meta.Public {
			return nil
		}
		rel, _ := filepath.Rel(notesDir, p)
		rel = filepath.ToSlash(rel)
		docs = append(docs, doc{
			path: rel,
			slug: slugify(strings.TrimSuffix(rel, ".md")),
			body: parsed.Body,
			Meta: parsed.Meta,
		})
		return nil
	})
	return docs, err
}

type navItem struct {
	Title string
	Href  string
}

func buildNav(pages []Page, base string) []navItem {
	items := make([]navItem, 0, len(pages))
	for _, p := range pages {
		items = append(items, navItem{Title: p.Title, Href: base + url.PathEscape(p.Slug) + ".html"})
	}
	return items
}

var slugRe = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "note"
	}
	if s == "home" {
		s = "index"
	}
	return s
}

// searchEntry is one page in the client-side search index.
type searchEntry struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

// writeSearchIndex emits search-index.json used by search.js.
func writeSearchIndex(outDir string, pages []Page, base string) error {
	entries := make([]searchEntry, 0, len(pages))
	for _, p := range pages {
		text := searchText(string(p.HTML))
		if len(text) > 4000 {
			text = text[:4000]
		}
		entries = append(entries, searchEntry{
			Title: p.Title,
			URL:   base + url.PathEscape(p.Slug) + ".html",
			Text:  text,
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "search-index.json"), data, 0o644)
}

// copyDir recursively copies a directory tree into dst.
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		in.Close()
		if closeErr := out.Close(); copyErr == nil {
			copyErr = closeErr
		}
		return copyErr
	})
}
