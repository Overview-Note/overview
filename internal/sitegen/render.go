package sitegen

import (
	"bytes"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// mdConverter parses vault Markdown with GFM (tables, strikethrough, linkify,
// task lists). Heading IDs are assigned later so they match the app's slug
// algorithm.
var mdConverter = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(goldhtml.WithUnsafe()),
)

func convertMarkdown(src string) (*xhtml.Node, error) {
	var buf bytes.Buffer
	if err := mdConverter.Convert([]byte(src), &buf); err != nil {
		return nil, err
	}
	nodes, err := xhtml.ParseFragment(strings.NewReader(buf.String()), &xhtml.Node{
		Type:     xhtml.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return nil, err
	}
	container := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		container.AppendChild(n)
	}
	return container, nil
}

// renderResult is the output of the Markdown -> app-compatible HTML pipeline.
type renderResult struct {
	HTML       string
	Headings   []Heading
	HasCode    bool
	HasMath    bool
	HasMermaid bool
}

// wikiLinkResolver resolves a wiki target to a site-relative href.
type wikiLinkResolver func(target string) (href string, ok bool)

// renderNote converts vault Markdown to the same HTML structure the Tiptap
// editor produces for its reading state. KaTeX and Mermaid are not rendered
// here: the markup keeps the raw source (data-tex / data-source) and the
// vendored runtime enhancement renders it in the browser.
func renderNote(markdownSrc string, resolve wikiLinkResolver, assetBase string) renderResult {
	var res renderResult
	prepared := mathToHTML(wikiToHTML(footnotesToHTML(markdownSrc), resolve), &res)
	parsed, err := convertMarkdown(prepared)
	if err != nil {
		return renderResult{HTML: prepared}
	}
	return postProcess(parsed, assetBase, res)
}

var (
	footnoteDefRe  = regexp.MustCompile(`^\[\^([^\]]+)\]:\s?`)
	footnoteRefRe  = regexp.MustCompile(`\[\^([^\]]+)\]`)
	wikiLinkRe     = regexp.MustCompile(`\[\[([^\[\]]+?)\]\]`)
	codeSegmentRe  = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")
	mathBlockRe    = regexp.MustCompile(`\$\$([\s\S]+?)\$\$`)
	mathInlineRe   = regexp.MustCompile(`(^|[^\\$])\$([^\n$]+?)\$`)
	headingStripRe = regexp.MustCompile(`[^\p{L}\p{N}\s-]+`)
	spaceRe        = regexp.MustCompile(`\s+`)
)

type footnoteDef struct {
	id   string
	text string
}

// footnotesToHTML mirrors web/src/markdown/footnotes.ts: definitions are moved
// to a trailing .fn-defs block and references become sup.fn-ref[data-id].
func footnotesToHTML(md string) string {
	lines := strings.Split(md, "\n")
	defs := make([]footnoteDef, 0, 4)
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		m := footnoteDefRe.FindStringSubmatchIndex(line)
		if m == nil || m[0] != 0 {
			body = append(body, line)
			continue
		}
		id := line[m[2]:m[3]]
		text := line[m[1]:]
		defs = append(defs, footnoteDef{id: id, text: strings.TrimRight(text, " \t\r")})
	}
	if len(defs) == 0 {
		return md
	}
	out := footnoteRefRe.ReplaceAllStringFunc(strings.Join(body, "\n"), func(match string) string {
		id := footnoteRefRe.FindStringSubmatch(match)[1]
		found := false
		for _, d := range defs {
			if d.id == id {
				found = true
				break
			}
		}
		if !found {
			return match
		}
		return `<sup class="fn-ref" data-id="` + escapeAttr(id) + `">[` + escapeHTML(id) + `]</sup>`
	})
	var b strings.Builder
	b.WriteString(out)
	b.WriteString("\n\n<div class=\"fn-defs\">")
	for i, d := range defs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(`<div class="fn-def" data-id="` + escapeAttr(d.id) + `">` + escapeHTML(d.text) + `</div>`)
	}
	b.WriteString("</div>")
	return b.String()
}

// wikiToHTML mirrors web/src/markdown/wiki.ts but resolves targets to real
// pages. Missing targets degrade to a non-link span, matching the app's
// unresolved state.
func wikiToHTML(md string, resolve wikiLinkResolver) string {
	return wikiLinkRe.ReplaceAllStringFunc(md, func(match string) string {
		inner := wikiLinkRe.FindStringSubmatch(match)[1]
		target := inner
		display := inner
		if i := strings.IndexByte(inner, '|'); i >= 0 {
			target = inner[:i]
			display = inner[i+1:]
		}
		target = strings.TrimSpace(target)
		display = strings.TrimSpace(display)
		if target == "" {
			return match
		}
		if display == "" {
			display = target
		}
		if href, ok := resolve(target); ok {
			return `<a class="wiki-link" href="` + escapeAttr(href) + `">` + escapeHTML(display) + `</a>`
		}
		return `<span class="wiki-link wiki-missing">` + escapeHTML(display) + `</span>`
	})
}

// mathToHTML mirrors web/src/markdown/math.ts, but keeps the raw TeX in
// data-tex instead of rendering KaTeX (the runtime does that).
func mathToHTML(md string, res *renderResult) string {
	var b strings.Builder
	last := 0
	for _, loc := range codeSegmentRe.FindAllStringIndex(md, -1) {
		b.WriteString(convertMathSegment(md[last:loc[0]], res))
		b.WriteString(md[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(convertMathSegment(md[last:], res))
	return b.String()
}

func convertMathSegment(seg string, res *renderResult) string {
	seg = mathBlockRe.ReplaceAllStringFunc(seg, func(match string) string {
		tex := strings.TrimSpace(mathBlockRe.FindStringSubmatch(match)[1])
		res.HasMath = true
		return `<div class="math-block" data-tex="` + escapeAttr(tex) + `">` + escapeHTML(tex) + `</div>`
	})
	seg = mathInlineRe.ReplaceAllStringFunc(seg, func(match string) string {
		sm := mathInlineRe.FindStringSubmatch(match)
		tex := strings.TrimSpace(sm[2])
		res.HasMath = true
		return sm[1] + `<span class="math-inline" data-tex="` + escapeAttr(tex) + `">` + escapeHTML(tex) + `</span>`
	})
	return seg
}

func escapeHTML(s string) string { return html.EscapeString(s) }

func escapeAttr(s string) string { return html.EscapeString(s) }

// findViewById returns the first descendant element with the given tag.
func findChild(n *xhtml.Node, tag string) *xhtml.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == xhtml.ElementNode && c.Data == tag {
			return c
		}
	}
	return nil
}

func element(tag string, attrs ...xhtml.Attribute) *xhtml.Node {
	return &xhtml.Node{Type: xhtml.ElementNode, Data: tag, DataAtom: atom.Lookup([]byte(tag)), Attr: attrs}
}

func textNode(s string) *xhtml.Node {
	return &xhtml.Node{Type: xhtml.TextNode, Data: s}
}

func attr(key, value string) xhtml.Attribute { return xhtml.Attribute{Key: key, Val: value} }

func getAttr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func setAttr(n *xhtml.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, attr(key, value))
}

func addClass(n *xhtml.Node, class string) {
	if cur, ok := getAttr(n, "class"); ok {
		for _, c := range strings.Fields(cur) {
			if c == class {
				return
			}
		}
		setAttr(n, "class", cur+" "+class)
		return
	}
	setAttr(n, "class", class)
}

func hasClass(n *xhtml.Node, class string) bool {
	cur, ok := getAttr(n, "class")
	if !ok {
		return false
	}
	for _, c := range strings.Fields(cur) {
		if c == class {
			return true
		}
	}
	return false
}

func hasAttr(n *xhtml.Node, key string) bool {
	_, ok := getAttr(n, key)
	return ok
}

func replaceNode(old, replacement *xhtml.Node) {
	parent := old.Parent
	if parent == nil {
		return
	}
	parent.InsertBefore(replacement, old)
	parent.RemoveChild(old)
}

// collect gathers every element node with one of the given tags in document
// order.
func collect(root *xhtml.Node, tags ...string) []*xhtml.Node {
	want := make(map[string]bool, len(tags))
	for _, t := range tags {
		want[t] = true
	}
	var out []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.ElementNode && want[c.Data] {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(root)
	return out
}

func nodeText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.TextNode {
				b.WriteString(c.Data)
			}
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func trimCodeText(code *xhtml.Node) {
	for c := code.LastChild; c != nil; c = c.PrevSibling {
		if c.Type == xhtml.TextNode {
			c.Data = strings.TrimSuffix(c.Data, "\n")
			return
		}
	}
}

// innerHTML renders a node's children the way the browser serializes a
// fragment, so entity-bearing heading text (e.g. "A &amp; B") is reproduced
// exactly rather than decoded to "A & B".
func innerHTML(n *xhtml.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		_ = xhtml.Render(&b, c)
	}
	return b.String()
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// slugHeading mirrors slugify in web/src/markdown/doc.ts. doc.ts slugs the raw
// rendered heading HTML (tags stripped, entities left encoded), so we do the
// same on the serialized inner HTML to keep `#id` / TOC anchors identical.
func slugHeading(innerHTML string) string {
	s := tagRe.ReplaceAllString(innerHTML, "")
	s = spaceRe.ReplaceAllString(s, " ")
	s = strings.ToLower(strings.TrimSpace(s))
	s = headingStripRe.ReplaceAllString(s, "")
	s = spaceRe.ReplaceAllString(s, "-")
	return s
}

func headingLevel(n *xhtml.Node) int {
	if n.Type != xhtml.ElementNode || len(n.Data) != 2 || n.Data[0] != 'h' {
		return 0
	}
	if n.Data[1] < '1' || n.Data[1] > '4' {
		return 0
	}
	return int(n.Data[1] - '0')
}

// postProcess rewrites goldmark's output into the app's reading DOM.
func postProcess(root *xhtml.Node, assetBase string, res renderResult) renderResult {
	assignHeadingIDs(root, &res)
	convertCodeBlocks(root, &res)
	convertTaskLists(root)
	wrapTableCells(root)
	resolveAssets(root, assetBase)
	var b strings.Builder
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		_ = xhtml.Render(&b, c)
	}
	res.HTML = b.String()
	return res
}

func assignHeadingIDs(root *xhtml.Node, res *renderResult) {
	counts := map[string]int{}
	headings := collect(root, "h1", "h2", "h3", "h4")
	for _, h := range headings {
		text := spaceRe.ReplaceAllString(strings.TrimSpace(nodeText(h)), " ")
		id := slugHeading(innerHTML(h))
		if id == "" {
			id = "section"
		}
		counts[id]++
		if counts[id] > 1 {
			id = id + "-" + strconv.Itoa(counts[id])
		}
		setAttr(h, "id", id)
		if level := headingLevel(h); level >= 2 {
			res.Headings = append(res.Headings, Heading{Level: level, ID: id, Text: text})
		}
	}
}

func convertCodeBlocks(root *xhtml.Node, res *renderResult) {
	for _, pre := range collect(root, "pre") {
		code := findChild(pre, "code")
		if code == nil {
			continue
		}
		trimCodeText(code)
		if hasClass(code, "language-mermaid") {
			source := nodeText(code)
			div := element("div", attr("class", "mermaid"), attr("data-source", source))
			div.AppendChild(textNode(source))
			replaceNode(pre, div)
			res.HasMermaid = true
			continue
		}
		addClass(pre, "code-block")
		res.HasCode = true
	}
}

func directCheckbox(li *xhtml.Node) *xhtml.Node {
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode || c.Data != "input" {
			continue
		}
		if t, _ := getAttr(c, "type"); strings.EqualFold(t, "checkbox") {
			return c
		}
	}
	return nil
}

func convertTaskLists(root *xhtml.Node) {
	lists := collect(root, "ul")
	for i := len(lists) - 1; i >= 0; i-- {
		ul := lists[i]
		var items []*xhtml.Node
		allTasks := true
		for c := ul.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != xhtml.ElementNode || c.Data != "li" {
				continue
			}
			items = append(items, c)
			if directCheckbox(c) == nil {
				allTasks = false
			}
		}
		if len(items) == 0 || !allTasks {
			continue
		}
		setAttr(ul, "data-type", "taskList")
		for _, li := range items {
			input := directCheckbox(li)
			checked := hasAttr(input, "checked")
			label := element("label")
			box := element("input", attr("type", "checkbox"))
			if checked {
				box.Attr = append(box.Attr, attr("checked", "checked"))
			}
			box.Attr = append(box.Attr, attr("disabled", ""))
			label.AppendChild(box)
			label.AppendChild(element("span"))

			div := element("div")
			p := element("p")
			var nested []*xhtml.Node
			for c := li.FirstChild; c != nil; {
				next := c.NextSibling
				li.RemoveChild(c)
				if c.Type == xhtml.ElementNode && (c.Data == "ul" || c.Data == "ol") {
					nested = append(nested, c)
				} else if c != input {
					p.AppendChild(c)
				}
				c = next
			}
			trimNodeEdges(p)
			div.AppendChild(p)
			for _, n := range nested {
				div.AppendChild(n)
			}
			li.AppendChild(label)
			li.AppendChild(div)
			setAttr(li, "data-type", "taskItem")
			if checked {
				setAttr(li, "data-checked", "true")
			} else {
				setAttr(li, "data-checked", "false")
			}
		}
	}
}

// trimNodeEdges drops whitespace-only leading/trailing text children so the
// rebuilt <p> matches the app's trimmed item text.
func trimNodeEdges(n *xhtml.Node) {
	for n.FirstChild != nil && n.FirstChild.Type == xhtml.TextNode && strings.TrimSpace(n.FirstChild.Data) == "" {
		n.RemoveChild(n.FirstChild)
	}
	for n.LastChild != nil && n.LastChild.Type == xhtml.TextNode && strings.TrimSpace(n.LastChild.Data) == "" {
		n.RemoveChild(n.LastChild)
	}
	if n.FirstChild != nil && n.FirstChild.Type == xhtml.TextNode {
		n.FirstChild.Data = strings.TrimLeft(n.FirstChild.Data, " \t\r\n")
	}
	if n.LastChild != nil && n.LastChild.Type == xhtml.TextNode {
		n.LastChild.Data = strings.TrimRight(n.LastChild.Data, " \t\r\n")
	}
}

func wrapTableCells(root *xhtml.Node) {
	for _, cell := range collect(root, "th", "td") {
		if cell.FirstChild != nil && cell.FirstChild == cell.LastChild &&
			cell.FirstChild.Type == xhtml.ElementNode && cell.FirstChild.Data == "p" {
			continue
		}
		p := element("p")
		for c := cell.FirstChild; c != nil; {
			next := c.NextSibling
			cell.RemoveChild(c)
			p.AppendChild(c)
			c = next
		}
		cell.AppendChild(p)
	}
}

var assetPrefixes = []string{"/api/v1/assets/", "/api/assets/", "/assets/", "assets/"}

func rewriteAssetURL(value, assetBase string) string {
	for _, prefix := range assetPrefixes {
		if strings.HasPrefix(value, prefix) {
			return assetBase + value[len(prefix):]
		}
	}
	return value
}

func resolveAssets(root *xhtml.Node, assetBase string) {
	for _, img := range collect(root, "img") {
		if src, ok := getAttr(img, "src"); ok {
			setAttr(img, "src", rewriteAssetURL(src, assetBase))
		}
		if !hasAttr(img, "loading") {
			setAttr(img, "loading", "lazy")
		}
		if !hasAttr(img, "decoding") {
			setAttr(img, "decoding", "async")
		}
	}
	for _, a := range collect(root, "a") {
		if href, ok := getAttr(a, "href"); ok {
			setAttr(a, "href", rewriteAssetURL(href, assetBase))
		}
	}
}

// searchText extracts indexable text, including code source and the raw TeX /
// Mermaid sources kept in data attributes.
func searchText(htmlSource string) string {
	doc, err := xhtml.Parse(strings.NewReader(htmlSource))
	if err != nil {
		return strings.TrimSpace(spaceRe.ReplaceAllString(htmlSource, " "))
	}
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.ElementNode {
				if tex, ok := getAttr(c, "data-tex"); ok {
					b.WriteString(" ")
					b.WriteString(tex)
					b.WriteString(" ")
					continue
				}
				if src, ok := getAttr(c, "data-source"); ok {
					b.WriteString(" ")
					b.WriteString(src)
					b.WriteString(" ")
					continue
				}
			}
			if c.Type == xhtml.TextNode {
				b.WriteString(c.Data)
			}
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(spaceRe.ReplaceAllString(b.String(), " "))
}
