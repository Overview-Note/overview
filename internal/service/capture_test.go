package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPreviewExtractsMetadata(t *testing.T) {
	const page = `<!doctype html><html><head>
<meta property="og:title" content="Hello &amp; Welcome">
<meta name="description" content="fallback description">
<title>ignored</title>
<style>body{color:red}</style>
<script>var x = 1;</script>
</head><body>
<h1>Heading</h1>
<p>Some body text with   lots
of whitespace.</p>
</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	defer ts.Close()

	allowPrivateNetworks = true
	defer func() { allowPrivateNetworks = false }()

	got, err := FetchPreview(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchPreview: %v", err)
	}
	if got.Title != "Hello & Welcome" {
		t.Errorf("title = %q, want %q", got.Title, "Hello & Welcome")
	}
	if !strings.Contains(got.Text, "Some body text") || !strings.Contains(got.Text, "whitespace.") {
		t.Errorf("text = %q", got.Text)
	}
	if strings.Contains(got.Text, "color:red") || strings.Contains(got.Text, "var x") {
		t.Errorf("script/style leaked into text: %q", got.Text)
	}
	if got.URL != ts.URL {
		t.Errorf("url = %q, want %q", got.URL, ts.URL)
	}
}

func TestFetchPreviewFallsBackToDescription(t *testing.T) {
	const page = `<html><head><title>Doc</title>
<meta name="description" content="only a description here">
</head><body><script>nothing()</script></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(page))
	}))
	defer ts.Close()

	allowPrivateNetworks = true
	defer func() { allowPrivateNetworks = false }()

	got, err := FetchPreview(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchPreview: %v", err)
	}
	if got.Title != "Doc" {
		t.Errorf("title = %q, want Doc", got.Title)
	}
	if got.Text != "only a description here" {
		t.Errorf("text = %q", got.Text)
	}
}

func TestFetchPreviewRejectsPrivateAndInvalid(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:5230/",
		"http://localhost/",
		"http://[::1]/",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"ftp://example.com/file",
		"not a url",
		"",
	}
	for _, raw := range cases {
		if _, err := FetchPreview(context.Background(), raw); err == nil {
			t.Errorf("FetchPreview(%q) = nil error, want rejection", raw)
		}
	}
}
