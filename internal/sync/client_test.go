package sync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeServerURL(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want string
	}{
		{"https://notes.example.com", true, "https://notes.example.com"},
		{"https://notes.example.com/", true, "https://notes.example.com"},
		{"http://localhost:5230", true, "http://localhost:5230"},
		{"http://127.0.0.1:5230", true, "http://127.0.0.1:5230"},
		{"http://[::1]:5230", true, "http://[::1]:5230"},
		{"http://example.com", false, ""},
		{"http://192.168.1.10:5230", false, ""},
		{"ftp://example.com", false, ""},
		{"", false, ""},
		{"not a url", false, ""},
	}
	for _, tc := range cases {
		got, err := normalizeServerURL(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Errorf("normalizeServerURL(%q) = (%q, %v), want %q", tc.in, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Errorf("normalizeServerURL(%q) succeeded, want error", tc.in)
		}
	}
}

func TestNewRemoteClientRejectsPlainHTTPRemote(t *testing.T) {
	if _, err := NewRemoteClient("http://notes.example.com", ClientOptions{}); err == nil {
		t.Fatal("plain http remote host accepted")
	}
}

func TestRemoteClientSendsAuthorization(t *testing.T) {
	var auth, ua string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		ua = r.Header.Get("User-Agent")
		w.Header().Set("ETag", `"v1"`)
		_ = json.NewEncoder(w).Encode(map[string]any{"vaultId": "v", "etag": "v1", "notes": []any{}, "folders": []any{}})
	}))
	defer ts.Close()

	client, err := NewRemoteClient(ts.URL, ClientOptions{Token: "tok123", UserAgent: "Test/1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.Manifest(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok123" {
		t.Errorf("Authorization = %q, want Bearer tok123", auth)
	}
	if ua != "Test/1" {
		t.Errorf("User-Agent = %q, want Test/1", ua)
	}
}

func TestRemoteClientManifestNotModified(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		t.Errorf("If-None-Match = %q", r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client, err := NewRemoteClient(ts.URL, ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	manifest, notModified, err := client.Manifest(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if !notModified || manifest != nil {
		t.Errorf("manifest = %v, notModified = %v, want nil,true", manifest, notModified)
	}
}

func TestRemoteClientPutRejectsEmptyBaseVersion(t *testing.T) {
	client, err := NewRemoteClient("https://example.com", ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PutNoteRaw(context.Background(), "a.md", []byte("x"), "")
	if err == nil {
		t.Fatal("empty baseVersion accepted")
	}
}

func TestHTTPErrorUnwrap(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusConflict, ErrConflict},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusNotImplemented, ErrNotSupported},
		{http.StatusRequestEntityTooLarge, ErrTooLarge},
		{http.StatusInternalServerError, ErrServer},
		{http.StatusBadGateway, ErrServer},
	}
	for _, tc := range cases {
		err := &HTTPError{Status: tc.status}
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d unwrap = %v, want %v", tc.status, err, tc.want)
		}
	}
}
