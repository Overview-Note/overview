package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func syncChanges(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, body := doJSON(t, http.MethodGet, url, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync/changes = %d, body = %v", resp.StatusCode, body)
	}
	return body
}

func changesList(t *testing.T, body map[string]any, key string) []any {
	t.Helper()
	list, ok := body[key].([]any)
	if !ok {
		t.Fatalf("%s missing or not a list: %v", key, body)
	}
	return list
}

func TestSyncChangesFullAndIncremental(t *testing.T) {
	ts := newTestServer(t)

	if resp, body := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]any{
		"path": "a.md", "body": "alpha", "baseVersion": "*",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create = %d %v", resp.StatusCode, body)
	}

	full := syncChanges(t, ts.URL+"/api/v1/sync/changes?since=0")
	changes := changesList(t, full, "changes")
	if len(changes) != 1 {
		t.Fatalf("full changes = %d, want 1", len(changes))
	}
	first := changes[0].(map[string]any)
	if first["path"] != "a.md" || first["op"] != "upsert" || first["version"] == "" {
		t.Errorf("unexpected change: %v", first)
	}
	if full["hasMore"] != false {
		t.Errorf("hasMore = %v, want false", full["hasMore"])
	}
	latest := int64(full["latestSeq"].(float64))
	if latest <= 0 {
		t.Fatalf("latestSeq = %d, want > 0", latest)
	}

	// Nothing new since the head.
	quiet := syncChanges(t, ts.URL+"/api/v1/sync/changes?since="+itoa(latest))
	if got := changesList(t, quiet, "changes"); len(got) != 0 {
		t.Errorf("changes after head = %d, want 0", len(got))
	}

	// A fresh write shows up only above the previous head.
	if resp, body := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]any{
		"path": "b.md", "body": "beta", "baseVersion": "*",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create b = %d %v", resp.StatusCode, body)
	}
	inc := syncChanges(t, ts.URL+"/api/v1/sync/changes?since="+itoa(latest))
	got := changesList(t, inc, "changes")
	if len(got) != 1 || got[0].(map[string]any)["path"] != "b.md" {
		t.Fatalf("incremental changes = %v, want [b.md]", got)
	}
}

func TestSyncChangesLimitAndHasMore(t *testing.T) {
	ts := newTestServer(t)
	for _, p := range []string{"a.md", "b.md", "c.md"} {
		if resp, body := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]any{
			"path": p, "body": p, "baseVersion": "*",
		}); resp.StatusCode != http.StatusOK {
			t.Fatalf("create %s = %d %v", p, resp.StatusCode, body)
		}
	}

	page := syncChanges(t, ts.URL+"/api/v1/sync/changes?since=0&limit=2")
	changes := changesList(t, page, "changes")
	if len(changes) != 2 || page["hasMore"] != true {
		t.Fatalf("page = %d hasMore=%v, want 2/true", len(changes), page["hasMore"])
	}
	lastSeq := int64(changes[1].(map[string]any)["seq"].(float64))

	next := syncChanges(t, ts.URL+"/api/v1/sync/changes?since="+itoa(lastSeq)+"&limit=2")
	rest := changesList(t, next, "changes")
	if len(rest) != 1 || next["hasMore"] != false {
		t.Fatalf("next page = %d hasMore=%v, want 1/false", len(rest), next["hasMore"])
	}
}

func TestSyncChangesFolderTombstone(t *testing.T) {
	ts := newTestServer(t)
	if resp, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/folder", map[string]any{"path": "empty"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("mkdir = %d %v", resp.StatusCode, body)
	}

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/note?path=empty", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete folder = %d", resp.StatusCode)
	}

	body := syncChanges(t, ts.URL+"/api/v1/sync/changes?since=0&sinceTs=0001-01-01T00:00:00Z")
	folders := changesList(t, body, "folderTombstones")
	if len(folders) != 1 || folders[0].(map[string]any)["path"] != "empty" {
		t.Fatalf("folderTombstones = %v, want [empty]", folders)
	}
	deletedAt, _ := folders[0].(map[string]any)["deletedAt"].(string)
	if deletedAt == "" {
		t.Errorf("folder tombstone missing deletedAt: %v", folders[0])
	}

	// A later cursor excludes the marker.
	after, err := time.Parse(time.RFC3339, deletedAt)
	if err != nil {
		t.Fatal(err)
	}
	filtered := syncChanges(t, ts.URL+"/api/v1/sync/changes?sinceTs="+after.Add(time.Second).Format(time.RFC3339))
	if got := changesList(t, filtered, "folderTombstones"); len(got) != 0 {
		t.Errorf("folderTombstones after cursor = %v, want none", got)
	}
}

func TestSyncChangesRejectsBadParams(t *testing.T) {
	ts := newTestServer(t)
	for _, q := range []string{"since=-1", "since=abc", "limit=0", "sinceTs=notatime"} {
		resp, _ := doJSON(t, http.MethodGet, ts.URL+"/api/v1/sync/changes?"+q, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", q, resp.StatusCode)
		}
	}
}

func TestSyncEventsStreamsChange(t *testing.T) {
	ts := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/sync/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	reader := bufio.NewReader(resp.Body)
	// The initial event announces the current cursor.
	initial := readSSEData(t, reader)
	if !strings.Contains(initial, "latestSeq") {
		t.Fatalf("initial event = %q, want latestSeq", initial)
	}

	// Trigger a write and expect a follow-up change event.
	if resp, body := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]any{
		"path": "live.md", "body": "live", "baseVersion": "*",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("create = %d %v", resp.StatusCode, body)
	}

	event := readSSEData(t, reader)
	var payload struct {
		LatestSeq int64 `json:"latestSeq"`
	}
	if err := json.Unmarshal([]byte(event), &payload); err != nil {
		t.Fatalf("decode event %q: %v", event, err)
	}
	if payload.LatestSeq <= 0 {
		t.Errorf("change event latestSeq = %d, want > 0", payload.LatestSeq)
	}
}

func readSSEData(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read sse: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if data != "" {
				return data
			}
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
