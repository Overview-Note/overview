package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/overview-app/overview/internal/index"
	"github.com/overview-app/overview/internal/server"
	"github.com/overview-app/overview/internal/service"
	"github.com/overview-app/overview/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	svc := service.New(st, ix, st)
	ts := httptest.NewServer(server.New(svc, server.Options{MaxUploadBytes: 1 << 20}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, method, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestNoteLifecycle(t *testing.T) {
	ts := newTestServer(t)

	resp, created := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "技术/Go/并发.md", "body": "# 并发模型\n\ngoroutine 与 channel", "baseVersion": "*",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, body = %v", resp.StatusCode, created)
	}
	version, _ := created["version"].(string)
	if version == "" {
		t.Fatalf("missing version: %v", created)
	}

	// stale write -> 409
	resp, _ = doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "技术/Go/并发.md", "body": "changed", "baseVersion": "stale",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("stale write status = %d, want 409", resp.StatusCode)
	}

	// create over existing -> 409
	resp, _ = doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "技术/Go/并发.md", "body": "x", "baseVersion": "*",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("create-over status = %d, want 409", resp.StatusCode)
	}

	// get with etag
	getURL := ts.URL + "/api/v1/note?path=" + url.QueryEscape("技术/Go/并发.md")
	resp, note := doJSON(t, http.MethodGet, getURL, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("missing ETag header")
	}
	if note["title"] != "并发模型" {
		t.Errorf("title = %v", note["title"])
	}

	// tree
	resp, tree := doJSON(t, http.MethodGet, ts.URL+"/api/v1/tree", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tree status = %d", resp.StatusCode)
	}
	if _, ok := tree["nodes"]; !ok {
		t.Errorf("tree missing nodes: %v", tree)
	}

	// search (CJK infix)
	resp, res := doJSON(t, http.MethodGet, ts.URL+"/api/v1/search?q="+url.QueryEscape("发模"), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d", resp.StatusCode)
	}
	if arr, _ := res["results"].([]any); len(arr) != 1 {
		t.Errorf("search results = %v", res["results"])
	}
}

func TestWikiLinksEndpoint(t *testing.T) {
	ts := newTestServer(t)
	if resp, b := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "dir/b.md", "body": "# Bee\n\nhello", "baseVersion": "*",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create B: %d %v", resp.StatusCode, b)
	}
	if resp, b := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "a.md", "body": "# A\n\nsee [[Bee]]", "baseVersion": "*",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create A: %d %v", resp.StatusCode, b)
	}

	resp, links := doJSON(t, http.MethodGet, ts.URL+"/api/v1/links?path="+url.QueryEscape("a.md"), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("links status = %d", resp.StatusCode)
	}
	outgoing, _ := links["outgoing"].([]any)
	if len(outgoing) != 1 {
		t.Fatalf("outgoing = %v", links["outgoing"])
	}
	first, _ := outgoing[0].(map[string]any)
	target, _ := first["target"].(map[string]any)
	if target == nil || target["path"] != "dir/b.md" {
		t.Errorf("target not resolved: %v", first)
	}

	_, back := doJSON(t, http.MethodGet, ts.URL+"/api/v1/links?path="+url.QueryEscape("dir/b.md"), nil)
	backlinks, _ := back["backlinks"].([]any)
	if len(backlinks) != 1 {
		t.Errorf("backlinks = %v", back["backlinks"])
	}

	resp, resolved := doJSON(t, http.MethodGet, ts.URL+"/api/v1/resolve?target="+url.QueryEscape("Bee"), nil)
	if resp.StatusCode != http.StatusOK || resolved["path"] != "dir/b.md" {
		t.Errorf("resolve = %d %v", resp.StatusCode, resolved)
	}
}

func TestPathValidation(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := doJSON(t, http.MethodGet, ts.URL+"/api/v1/note?path="+url.QueryEscape("../secret"), nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("escape path status = %d, want 400", resp.StatusCode)
	}
}

func TestAssetUploadAndServe(t *testing.T) {
	ts := newTestServer(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "pic.png")
	_, _ = fw.Write([]byte("PNGDATA"))
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/assets", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var asset map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&asset)
	if resp.StatusCode != http.StatusOK || asset["url"] == "" {
		t.Fatalf("upload failed: %d %v", resp.StatusCode, asset)
	}

	got, err := http.Get(ts.URL + asset["url"])
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("asset status = %d", got.StatusCode)
	}
	if got.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff header")
	}
	body, _ := io.ReadAll(got.Body)
	if strings.TrimSpace(string(body)) != "PNGDATA" {
		t.Errorf("asset body = %q", body)
	}
}
