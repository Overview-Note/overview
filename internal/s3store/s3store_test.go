package s3store

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// decodeAWSChunked unwraps the SigV4 streaming (aws-chunked) body minio-go
// sends, so the test can assert the decoded payload.
func decodeAWSChunked(t *testing.T, body string) string {
	t.Helper()
	var out strings.Builder
	for {
		idx := strings.Index(body, "\r\n")
		if idx < 0 {
			t.Fatalf("malformed chunked body: %q", body)
		}
		sizeHex := body[:idx]
		if i := strings.IndexByte(sizeHex, ';'); i >= 0 {
			sizeHex = sizeHex[:i]
		}
		n, err := strconv.ParseInt(sizeHex, 16, 64)
		if err != nil {
			t.Fatalf("bad chunk size %q: %v", sizeHex, err)
		}
		body = body[idx+2:]
		if n == 0 {
			return out.String()
		}
		if int64(len(body)) < n {
			t.Fatalf("short chunk: have %d want %d", len(body), n)
		}
		out.WriteString(body[:n])
		body = body[n:]
		body = strings.TrimPrefix(body, "\r\n")
	}
}

// TestStoreRestorePutsObjectAtPath drives Restore against a fake S3 endpoint to
// confirm the object lands at the exact vault-relative key with its bytes
// intact.
func TestStoreRestorePutsObjectAtPath(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			gotMethod = r.Method
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			w.Header().Set("ETag", `"abc"`)
		case http.MethodHead:
			// Bucket existence probe.
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store, err := New(context.Background(), Options{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Region:    "us-east-1",
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "bucket",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := store.Restore(context.Background(), "2026/10/pic.png", strings.NewReader("payload")); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/bucket/2026/10/pic.png" {
		t.Errorf("key = %q, want /bucket/2026/10/pic.png", gotPath)
	}
	if decoded := decodeAWSChunked(t, gotBody); decoded != "payload" {
		t.Errorf("body = %q, want payload", decoded)
	}
}
