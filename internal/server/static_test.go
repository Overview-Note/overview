package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testStaticFS() fstest.MapFS {
	body := strings.Repeat("console.log('overview');\n", 200)
	return fstest.MapFS{
		"index.html":                &fstest.MapFile{Data: []byte("<!doctype html><div id=app></div>")},
		"assets/app-abc12345.js":    &fstest.MapFile{Data: []byte(body)},
		"assets/style-abc12345.css": &fstest.MapFile{Data: []byte("body{color:#000}")},
		"icon.svg":                  &fstest.MapFile{Data: []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")},
		"manifest.webmanifest":      &fstest.MapFile{Data: []byte(`{"name":"Overview"}`)},
	}
}

func staticFSWithAsset(assetPath, content string) fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><div id=app></div>")},
		assetPath:    &fstest.MapFile{Data: []byte(content)},
	}
}

func TestWeakETagContentSensitive(t *testing.T) {
	same := strings.Repeat("a", 256)
	etag := weakETag("assets/app-abc12345.js", []byte(same))
	if etag != weakETag("assets/app-abc12345.js", []byte(same)) {
		t.Fatal("identical content produced different ETags")
	}
	if etag == weakETag("assets/app-abc12345.js", []byte(strings.Repeat("b", 256))) {
		t.Fatal("same length, different content produced the same ETag")
	}
	if etag == weakETag("assets/other-abc12345.js", []byte(same)) {
		t.Fatal("different paths produced the same ETag")
	}
}

func TestStaticSameLengthDifferentContentRevalidates(t *testing.T) {
	const asset = "assets/app-abc12345.js"

	first := newStaticHandler(staticFSWithAsset(asset, strings.Repeat("a", 256)))
	rec := httptest.NewRecorder()
	first.serve(rec, httptest.NewRequest(http.MethodGet, "/"+asset, nil))
	etag := rec.Result().Header.Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}

	second := newStaticHandler(staticFSWithAsset(asset, strings.Repeat("b", 256)))
	req := httptest.NewRequest(http.MethodGet, "/"+asset, nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	second.serve(rec, req)

	resp := rec.Result()
	if resp.StatusCode == http.StatusNotModified {
		t.Fatal("different content of equal length was answered with 304 from a stale ETag")
	}
	if got := resp.Header.Get("ETag"); got == etag {
		t.Fatalf("ETag unchanged for different content: %q", got)
	}
}

func TestStaticMissingAssetReturns404(t *testing.T) {
	h := newStaticHandler(testStaticFS())
	// Bundled assets and whitelisted top-level files that are absent from the
	// build are genuine 404s, never masked by index.html.
	for _, p := range []string{"/assets/missing.js", "/favicon.ico", "/sitemap.xml"} {
		rec := httptest.NewRecorder()
		h.serve(rec, httptest.NewRequest(http.MethodGet, p, nil))
		resp := rec.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", p, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s Content-Type = %q, want non-html", p, ct)
		}
	}
}

func TestStaticNoteRoutesFallBackToSPA(t *testing.T) {
	h := newStaticHandler(testStaticFS())
	// Extension-bearing client routes must not be mistaken for assets: /note
	// and /public take arbitrary note paths that end in .md.
	for _, p := range []string{"/note/guide/start.md", "/public/notes/a.md", "/note/hello"} {
		rec := httptest.NewRecorder()
		h.serve(rec, httptest.NewRequest(http.MethodGet, p, nil))
		resp := rec.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want 200", p, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s Content-Type = %q, want text/html", p, ct)
		}
	}
}

func TestStaticTopLevelWhitelist(t *testing.T) {
	h := newStaticHandler(testStaticFS())

	// A whitelisted top-level file that exists is served directly.
	rec := httptest.NewRecorder()
	h.serve(rec, httptest.NewRequest(http.MethodGet, "/icon.svg", nil))
	if got := rec.Result().StatusCode; got != http.StatusOK {
		t.Errorf("/icon.svg status = %d, want 200", got)
	}

	// A whitelisted top-level file that does not exist is a 404, not a fallback.
	rec = httptest.NewRecorder()
	h.serve(rec, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if got := rec.Result().StatusCode; got != http.StatusNotFound {
		t.Errorf("/favicon.ico status = %d, want 404", got)
	}
}

func TestStaticClientRouteFallsBack(t *testing.T) {
	h := newStaticHandler(testStaticFS())
	rec := httptest.NewRecorder()
	h.serve(rec, httptest.NewRequest(http.MethodGet, "/some/route", nil))
	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/some/route status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("/some/route Content-Type = %q, want text/html", ct)
	}
}

func TestStaticAssetImmutableHeader(t *testing.T) {
	h := newStaticHandler(testStaticFS())
	rec := httptest.NewRecorder()
	h.serve(rec, httptest.NewRequest(http.MethodGet, "/assets/style-abc12345.css", nil))
	if cc := rec.Result().Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
}

func TestStaticCompressionAndCaching(t *testing.T) {
	h := newStaticHandler(testStaticFS())

	// Hashed assets: gzip + immutable cache.
	req := httptest.NewRequest(http.MethodGet, "/assets/app-abc12345.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.serve(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	raw, _ := io.ReadAll(zr)
	if !strings.Contains(string(raw), "console.log") {
		t.Errorf("decompressed body mismatch")
	}
}

func TestStaticConditionalRequest(t *testing.T) {
	h := newStaticHandler(testStaticFS())

	first := httptest.NewRecorder()
	h.serve(first, httptest.NewRequest(http.MethodGet, "/assets/style-abc12345.css", nil))
	etag := first.Result().Header.Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/style-abc12345.css", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	h.serve(rec, req)
	if rec.Result().StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rec.Result().StatusCode)
	}
}

func TestStaticSPAFallbackAndTypes(t *testing.T) {
	h := newStaticHandler(testStaticFS())

	// Unknown client route falls back to index.html and must not be cached.
	rec := httptest.NewRecorder()
	h.serve(rec, httptest.NewRequest(http.MethodGet, "/note/Some/Path", nil))
	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fallback status = %d", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index Cache-Control = %q, want no-cache", cc)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("index Content-Type = %q", ct)
	}

	req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec = httptest.NewRecorder()
	h.serve(rec, req)
	if ct := rec.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("manifest Content-Type = %q", ct)
	}
}
