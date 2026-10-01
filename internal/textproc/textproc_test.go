package textproc

import (
	"strings"
	"testing"
)

func TestTokensCJK(t *testing.T) {
	tokens := Tokens("并发模型")
	set := make(map[string]bool)
	for _, tk := range tokens {
		set[tk] = true
	}
	for _, want := range []string{"并", "发", "模", "型", "并发", "发模", "模型"} {
		if !set[want] {
			t.Errorf("expected token %q in %v", want, tokens)
		}
	}
}

func TestTokensMixed(t *testing.T) {
	tokens := Tokens("Go 并发 channel")
	joined := strings.Join(tokens, " ")
	for _, want := range []string{"go", "并发", "发", "channel"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in tokens %v", want, tokens)
		}
	}
}

func TestIsASCIIWord(t *testing.T) {
	if !IsASCIIWord("channel") {
		t.Error("channel should be ascii word")
	}
	if IsASCIIWord("并发") {
		t.Error("CJK should not be ascii word")
	}
}

func TestSnippetHighlights(t *testing.T) {
	got := Snippet("这是关于并发模型和 channel 的说明", "并发", 20)
	if !strings.Contains(got, "<mark>并发</mark>") {
		t.Errorf("expected highlight, got %q", got)
	}
}

func TestExtractWikiLinks(t *testing.T) {
	body := "参见 [[并发模型]] 与 [[Go 并发|Go]]，重复 [[并发模型]]。"
	got := ExtractWikiLinks(body)
	want := []string{"并发模型", "Go 并发"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractWikiLinksIgnoresCode(t *testing.T) {
	body := "`[[inline]]`\n\n```\n[[fenced]]\n```\n\n[[real]]"
	got := ExtractWikiLinks(body)
	if len(got) != 1 || got[0] != "real" {
		t.Errorf("got %v, want [real]", got)
	}
}

func TestSnippetEscapesHTML(t *testing.T) {
	got := Snippet(`<script>alert(1)</script> channel`, "channel", 60)
	if strings.Contains(got, "<script>") {
		t.Errorf("script tag must be escaped, got %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected escaped script, got %q", got)
	}
	if !strings.Contains(got, "<mark>channel</mark>") {
		t.Errorf("expected highlight, got %q", got)
	}
}
