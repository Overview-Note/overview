package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWriteNotFoundPage(t *testing.T) {
	rec := httptest.NewRecorder()
	writeNotFoundPage(rec)

	resp := rec.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "404") {
		t.Errorf("body does not contain 404: %q", body)
	}
}

func TestStaticMissingIndexServesNotFoundPage(t *testing.T) {
	// An FS without index.html models a frontend that was never built.
	h := newStaticHandler(fstest.MapFS{
		"assets/app-abc12345.js": &fstest.MapFile{Data: []byte("x")},
	})
	for _, p := range []string{"/", "/note/hello.md"} {
		rec := httptest.NewRecorder()
		h.serve(rec, httptest.NewRequest(http.MethodGet, p, nil))
		resp := rec.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", p, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s Content-Type = %q, want text/html", p, ct)
		}
	}
}
