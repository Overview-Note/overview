// Package sitegen exports a Markdown vault to a static, read-only HTML site.
// It powers `overview export`, which builds the project's documentation site
// from Overview notes so it can be hosted on any static host (e.g. GitHub Pages).
package sitegen

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/Overview-Note/overview/internal/markdown"
)

// Page is a rendered note.
type Page struct {
	Title    string
	Slug     string // file name without extension, e.g. "getting-started"
	Path     string // vault-relative source path
	HTML     template.HTML
	Headings []Heading
}

// Heading is an entry in a page's outline.
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Options configures the export.
type Options struct {
	NotesDir  string
	OutDir    string
	Base      string // URL base path, e.g. "/overview/" for GitHub Pages
	SiteTitle string
}

var wikiRe = regexp.MustCompile(`\[\[([^[\]]+?)\]\]`)

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

	docs, err := loadNotes(opts.NotesDir)
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

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	pages := make([]Page, 0, len(docs))
	for _, d := range docs {
		body := resolveWikiLinks(d.body, byTitle, byName, base)
		var buf bytes.Buffer
		if err := md.Convert([]byte(body), &buf); err != nil {
			return 0, fmt.Errorf("render %s: %w", d.path, err)
		}
		title := d.Meta.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(d.path), ".md")
		}
		pages = append(pages, Page{
			Title:    title,
			Slug:     d.slug,
			Path:     d.path,
			HTML:     template.HTML(buf.String()),
			Headings: extractHeadings(buf.String()),
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

	nav := buildNav(pages, base)
	for _, p := range pages {
		out := filepath.Join(opts.OutDir, p.Slug+".html")
		var buf bytes.Buffer
		if err := pageTmpl.Execute(&buf, map[string]any{
			"SiteTitle": opts.SiteTitle,
			"Title":     p.Title,
			"Base":      base,
			"Nav":       nav,
			"Content":   p.HTML,
			"Headings":  p.Headings,
			"IsHome":    false,
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

func loadNotes(notesDir string) ([]doc, error) {
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
		if !parsed.Meta.Public {
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

func resolveWikiLinks(body string, byTitle, byName map[string]string, base string) string {
	return wikiRe.ReplaceAllStringFunc(body, func(m string) string {
		inner := wikiRe.FindStringSubmatch(m)[1]
		target := inner
		display := inner
		if i := strings.IndexByte(inner, '|'); i >= 0 {
			target = inner[:i]
			display = inner[i+1:]
		}
		target = strings.TrimSpace(target)
		display = strings.TrimSpace(display)
		key := strings.ToLower(target)
		slug, ok := byTitle[key]
		if !ok {
			slug, ok = byName[key]
		}
		if !ok {
			return display
		}
		return fmt.Sprintf("[%s](%s%s.html)", display, base, url.PathEscape(slug))
	})
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

var headingRe = regexp.MustCompile(`<h([1-4]) id="([^"]+)">(.*?)</h[1-4]>`)
var tagRe = regexp.MustCompile(`<[^>]+>`)

func extractHeadings(html string) []Heading {
	var out []Heading
	for _, m := range headingRe.FindAllStringSubmatch(html, -1) {
		level := int(m[1][0] - '0')
		text := tagRe.ReplaceAllString(m[3], "")
		out = append(out, Heading{Level: level, ID: m[2], Text: htmlUnescape(text)})
	}
	return out
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&quot;", `"`)
	return r.Replace(s)
}
