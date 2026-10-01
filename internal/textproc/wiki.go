package textproc

import (
	"regexp"
	"strings"
)

var (
	fencedCode   = regexp.MustCompile("(?s)```.*?```")
	inlineCode   = regexp.MustCompile("`[^`\n]*`")
	wikiLinkExpr = regexp.MustCompile(`\[\[([^\[\]]+?)\]\]`)
)

// ExtractWikiLinks returns the unique targets referenced by [[wiki-links]] in
// body. Code blocks and inline code are ignored to reduce false positives.
// An optional alias after '|' is stripped: [[Target|Display]] -> Target.
func ExtractWikiLinks(body string) []string {
	cleaned := stripCode(body)
	matches := wikiLinkExpr.FindAllStringSubmatch(cleaned, -1)
	seen := make(map[string]struct{}, len(matches))
	targets := make([]string, 0, len(matches))
	for _, m := range matches {
		target := m[1]
		if i := strings.IndexByte(target, '|'); i >= 0 {
			target = target[:i]
		}
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

func stripCode(s string) string {
	s = fencedCode.ReplaceAllString(s, " ")
	return inlineCode.ReplaceAllString(s, " ")
}
