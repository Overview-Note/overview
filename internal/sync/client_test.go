package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestRemoteClientChanges(t *testing.T) {
	var gotSince, gotSinceTs, gotLimit string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("since")
		gotSinceTs = r.URL.Query().Get("sinceTs")
		gotLimit = r.URL.Query().Get("limit")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"latestSeq":        7,
			"latestTs":         "2026-10-06T12:00:00Z",
			"hasMore":          false,
			"changes":          []any{},
			"tombstones":       []any{},
			"folderTombstones": []any{map[string]any{"path": "empty", "deletedAt": "2026-10-06T12:00:00Z"}},
		})
	}))
	defer ts.Close()

	client, err := NewRemoteClient(ts.URL, ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := client.Changes(context.Background(), 3, "2026-10-06T11:00:00Z", 25)
	if err != nil {
		t.Fatal(err)
	}
	if gotSince != "3" || gotSinceTs != "2026-10-06T11:00:00Z" || gotLimit != "25" {
		t.Errorf("query = since=%q sinceTs=%q limit=%q", gotSince, gotSinceTs, gotLimit)
	}
	if changes.LatestSeq != 7 {
		t.Errorf("latestSeq = %d, want 7", changes.LatestSeq)
	}
	if len(changes.FolderTombstones) != 1 || changes.FolderTombstones[0].Path != "empty" {
		t.Errorf("folderTombstones = %+v", changes.FolderTombstones)
	}
}

func TestRemoteClientEventsParsesHeartbeat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept = %q, want text/event-stream", r.Header.Get("Accept"))
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ": ping\n\n")
		fmt.Fprint(w, "event: change\ndata: {\"latestSeq\":4,\"latestTs\":\"2026-10-06T12:00:00Z\"}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer ts.Close()

	client, err := NewRemoteClient(ts.URL, ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type ev struct {
		seq int
		ts  string
	}
	got := make(chan ev, 4)
	go func() {
		_ = client.Events(ctx, func(seq int, latestTs string) { got <- ev{seq, latestTs} })
	}()

	select {
	case e := <-got:
		if e.seq != 4 || e.ts != "2026-10-06T12:00:00Z" {
			t.Fatalf("event = %+v", e)
		}
	case <-ctx.Done():
		t.Fatal("no change event received")
	}
}

func TestRemoteClientEventsReconnects(t *testing.T) {
	var connections int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&connections, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "event: change\ndata: {\"latestSeq\":%d,\"latestTs\":\"\"}\n\n", n)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Return, closing the stream so the client must reconnect.
	}))
	defer ts.Close()

	client, err := NewRemoteClient(ts.URL, ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var mu sync.Mutex
	var seqs []int
	done := make(chan struct{})
	go func() {
		_ = client.Events(ctx, func(seq int, _ string) {
			mu.Lock()
			seqs = append(seqs, seq)
			n := len(seqs)
			mu.Unlock()
			if n >= 2 {
				select {
				case <-done:
				default:
					close(done)
				}
			}
		})
	}()

	select {
	case <-done:
	case <-ctx.Done():
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("client did not reconnect: events = %v, connections = %d", seqs, atomic.LoadInt32(&connections))
	}
	if atomic.LoadInt32(&connections) < 2 {
		t.Errorf("connections = %d, want >= 2", atomic.LoadInt32(&connections))
	}
}

func TestRemoteClientEventsUnauthorizedStops(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	client, err := NewRemoteClient(ts.URL, ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Events(context.Background(), func(int, string) {})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Events error = %v, want ErrUnauthorized", err)
	}
}

func TestRemoteClientChangesQueryEncoding(t *testing.T) {
	var raw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"latestSeq": 0, "latestTs": "0001-01-01T00:00:00Z"})
	}))
	defer ts.Close()
	client, _ := NewRemoteClient(ts.URL, ClientOptions{})
	if _, err := client.Changes(context.Background(), 0, "", 0); err != nil {
		t.Fatal(err)
	}
	if raw != "since=0" {
		t.Errorf("raw query = %q, want since=0", raw)
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
