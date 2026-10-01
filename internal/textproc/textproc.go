// Package textproc provides language-aware tokenization and snippet
// generation. It exists because SQLite FTS5's default tokenizers do not
// support substring search for CJK text, which is essential for a
// Chinese-friendly knowledge base.
package textproc

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

func isWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Tokens splits text into index/query tokens. Runs of CJK characters are
// expanded into overlapping unigrams and bigrams so that substring queries
// like "发模" (inside "并发模型") match, while Latin words are kept intact.
func Tokens(s string) []string {
	runes := []rune(s)
	tokens := make([]string, 0, len(runes))
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case isCJK(r):
			start := i
			for i < len(runes) && isCJK(runes[i]) {
				i++
			}
			run := runes[start:i]
			for j := range run {
				tokens = append(tokens, string(run[j])) // unigram
				if j+1 < len(run) {
					tokens = append(tokens, string(run[j:j+2])) // bigram
				}
			}
		case isWord(r):
			start := i
			for i < len(runes) && isWord(runes[i]) && !isCJK(runes[i]) {
				i++
			}
			tokens = append(tokens, strings.ToLower(string(runes[start:i])))
		default:
			i++
		}
	}
	return tokens
}

// Segment joins Tokens with spaces, producing text suitable for indexing into
// an FTS5 table using the unicode61 tokenizer.
func Segment(s string) string {
	return strings.Join(Tokens(s), " ")
}

// IsASCIIWord reports whether a token consists solely of ASCII letters/digits,
// in which case prefix matching is appropriate for search.
func IsASCIIWord(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if r > unicode.MaxASCII {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// RawTerms extracts whitespace-separated terms from a user query, used for
// highlighting in snippets.
func RawTerms(query string) []string {
	fields := strings.Fields(query)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			terms = append(terms, f)
		}
	}
	return terms
}

// Snippet returns an HTML-escaped excerpt of body around the first occurrence
// of any query term, with matches wrapped in <mark> tags. Because it is fully
// escaped, the result is safe to inject as HTML.
func Snippet(body, query string, radius int) string {
	if radius <= 0 {
		radius = 60
	}
	runes := []rune(body)
	lower := []rune(strings.ToLower(body))

	best, bestLen := -1, 0
	for _, term := range RawTerms(query) {
		needle := []rune(strings.ToLower(term))
		if len(needle) == 0 {
			continue
		}
		if idx := indexRunes(lower, needle); idx >= 0 && (best < 0 || idx < best) {
			best, bestLen = idx, len(needle)
		}
	}
	if best < 0 {
		best, bestLen = 0, 0
	}

	start := best - radius
	if start < 0 {
		start = 0
	}
	end := best + bestLen + radius
	if end > len(runes) {
		end = len(runes)
	}

	window := string(runes[start:end])
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	suffix := ""
	if end < len(runes) {
		suffix = "…"
	}
	return prefix + highlight(html.EscapeString(window), RawTerms(query)) + suffix
}

func highlight(escaped string, terms []string) string {
	patterns := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		patterns = append(patterns, regexp.QuoteMeta(t))
	}
	if len(patterns) == 0 {
		return escaped
	}
	re := regexp.MustCompile(`(?i)` + strings.Join(patterns, "|"))
	return re.ReplaceAllStringFunc(escaped, func(m string) string {
		return "<mark>" + m + "</mark>"
	})
}

func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
