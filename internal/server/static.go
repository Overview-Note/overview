package server

import (
	"bytes"
	"compress/gzip"
	"hash/fnv"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// staticHandler serves the embedded frontend with compression and sensible
// caching. Go's http.FileServer does neither, which leaves the multi-hundred-KB
// JS/CSS bundles uncompressed and re-fetched on every load.
type staticHandler struct {
	fsys    fs.FS
	gzipMu  sync.Mutex
	gzipMap map[string][]byte
}

func newStaticHandler(fsys fs.FS) *staticHandler {
	return &staticHandler{fsys: fsys, gzipMap: map[string][]byte{}}
}

func (h *staticHandler) serve(w http.ResponseWriter, r *http.Request) {
	// SPA fallback: unknown client routes serve index.html. Genuine asset
	// requests (bundled assets/ files or a top-level static file) must 404
	// instead, so a missing bundle is never masked by index.html served under
	// a JS/CSS content type.
	p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if p == "" {
		p = "index.html"
	}
	if _, err := fs.Stat(h.fsys, p); err != nil {
		if isAssetRequest(p) {
			http.NotFound(w, r)
			return
		}
		p = "index.html"
	}
	data, err := fs.ReadFile(h.fsys, p)
	if err != nil {
		// A missing index.html means the frontend was never built (or its
		// bundle is damaged): serve the self-contained HTML 404 rather than a
		// bare text/plain response. Asset 404s already returned above.
		if p == "index.html" {
			writeNotFoundPage(w)
			return
		}
		http.NotFound(w, r)
		return
	}

	ct := contentType(p)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Content-hashed assets are safe to cache forever; everything else
	// (index.html, sw.js, manifest, icons) must revalidate.
	if strings.HasPrefix(p, "assets/") && strings.Contains(p, "-") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	etag := weakETag(p, data)
	w.Header().Set("ETag", etag)
	if etagMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if gzipAccepted(r) && isCompressible(ct) {
		gz := h.gzip(p, data)
		if gz != nil {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			w.Header().Set("Content-Length", strconv.Itoa(len(gz)))
			_, _ = w.Write(gz)
			return
		}
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// gzip returns the cached gzip encoding of data, computing it on first use.
// It returns nil when compression does not help.
func (h *staticHandler) gzip(key string, data []byte) []byte {
	h.gzipMu.Lock()
	defer h.gzipMu.Unlock()
	if cached, ok := h.gzipMap[key]; ok {
		return cached
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()
	if buf.Len() >= len(data) {
		return nil
	}
	enc := buf.Bytes()
	h.gzipMap[key] = enc
	return enc
}

func contentType(p string) string {
	if strings.HasSuffix(p, ".webmanifest") {
		return "application/manifest+json"
	}
	for _, font := range []string{".woff2", ".woff", ".ttf"} {
		if strings.HasSuffix(p, font) {
			if ct := mime.TypeByExtension(font); ct != "" {
				return ct
			}
		}
	}
	if ct := mime.TypeByExtension(path.Ext(p)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func isCompressible(ct string) bool {
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	for _, t := range []string{
		"application/javascript",
		"application/json",
		"application/manifest+json",
		"application/xml",
		"image/svg+xml",
	} {
		if strings.HasPrefix(ct, t) {
			return true
		}
	}
	return false
}

func gzipAccepted(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func etagMatch(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		candidate := strings.TrimSpace(part)
		if candidate == etag || candidate == "*" || strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

func weakETag(p string, data []byte) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(p))
	_, _ = h.Write(data)
	return `W/"` + strconv.FormatUint(h.Sum64(), 36) + `"`
}

// staticRoots are the fixed top-level files the frontend ships. A request for
// a whitelisted name that is missing from the build is a genuine 404 rather
// than an SPA route. Anything not listed here (e.g. /note/hello.md or
// /public/hello.md) falls back to index.html and is owned by the client router.
var staticRoots = map[string]bool{
	"icon.svg":             true,
	"icon-192.png":         true,
	"icon-512.png":         true,
	"manifest.webmanifest": true,
	"sw.js":                true,
	"favicon.ico":          true,
	"robots.txt":           true,
	"sitemap.xml":          true,
}

// isAssetRequest reports whether p addresses a static asset rather than a
// client-side route. Missing bundled assets (assets/...) and missing top-level
// static files must return 404; every other unmatched path falls back to
// index.html so the SPA can own it.
func isAssetRequest(p string) bool {
	if strings.HasPrefix(p, "assets/") {
		return true
	}
	return staticRoots[p]
}
