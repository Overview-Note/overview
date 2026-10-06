package server_test

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestMkdirAdvancesCursorAndStreamsEvent verifies that creating an empty folder
// advances the observable change cursor (so an event-driven client reacts) and
// that the SSE stream announces it.
func TestMkdirAdvancesCursorAndStreamsEvent(t *testing.T) {
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
	reader := bufio.NewReader(resp.Body)
	if initial := readSSEData(t, reader); !strings.Contains(initial, "latestSeq") {
		t.Fatalf("initial event = %q, want latestSeq", initial)
	}

	before := syncChanges(t, ts.URL+"/api/v1/sync/changes")
	beforeSeq := int64(before["latestSeq"].(float64))

	if resp, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/folder", map[string]any{"path": "empty"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("mkdir = %d %v", resp.StatusCode, body)
	}

	after := syncChanges(t, ts.URL+"/api/v1/sync/changes")
	afterSeq := int64(after["latestSeq"].(float64))
	if afterSeq <= beforeSeq {
		t.Fatalf("latestSeq did not advance: %d -> %d", beforeSeq, afterSeq)
	}

	event := readSSEData(t, reader)
	if !strings.Contains(event, "latestSeq") {
		t.Errorf("change event = %q, want latestSeq", event)
	}
}
