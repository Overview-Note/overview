package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/mcp"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

func newMCPServer(t *testing.T, verify mcp.TokenVerifier) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	_ = os.MkdirAll(notes, 0o755)
	_ = os.MkdirAll(assets, 0o755)
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	svc := service.New(st, ix, st, nil, nil)
	ts := httptest.NewServer(mcp.New(svc, verify))
	t.Cleanup(ts.Close)
	return ts
}

func rpc(t *testing.T, ts *httptest.Server, token, method string, params any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func callTool(t *testing.T, ts *httptest.Server, token, name string, args map[string]any) (string, bool) {
	t.Helper()
	out := rpc(t, ts, token, "tools/call", map[string]any{"name": name, "arguments": args})
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", out)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("empty content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	isErr, _ := result["isError"].(bool)
	return text, isErr
}

func TestMCPInitializeAndList(t *testing.T) {
	ts := newMCPServer(t, nil)

	out := rpc(t, ts, "", "initialize", map[string]any{})
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize failed: %v", out)
	}
	if result["protocolVersion"] == "" {
		t.Errorf("missing protocolVersion: %v", result)
	}

	tools := rpc(t, ts, "", "tools/list", map[string]any{})
	tr, _ := tools["result"].(map[string]any)
	list, _ := tr["tools"].([]any)
	if len(list) < 5 {
		t.Errorf("expected several tools, got %d", len(list))
	}
}

func TestMCPTools(t *testing.T) {
	ts := newMCPServer(t, nil)

	text, isErr := callTool(t, ts, "", "notes_write", map[string]any{
		"path": "mcp/demo.md", "body": "# MCP 演示\n\ngoroutine channel", "baseVersion": "*",
	})
	if isErr {
		t.Fatalf("write failed: %s", text)
	}
	if !strings.Contains(text, "mcp/demo.md") {
		t.Errorf("write result = %s", text)
	}

	text, isErr = callTool(t, ts, "", "notes_read", map[string]any{"path": "mcp/demo.md"})
	if isErr || !strings.Contains(text, "goroutine") {
		t.Errorf("read = %s (err=%v)", text, isErr)
	}

	text, isErr = callTool(t, ts, "", "notes_search", map[string]any{"query": "channel"})
	if isErr || !strings.Contains(text, "mcp/demo.md") {
		t.Errorf("search = %s (err=%v)", text, isErr)
	}

	text, isErr = callTool(t, ts, "", "notes_list", nil)
	if isErr || !strings.Contains(text, "mcp") {
		t.Errorf("list = %s (err=%v)", text, isErr)
	}

	if _, isErr = callTool(t, ts, "", "notes_delete", map[string]any{"path": "mcp/demo.md"}); isErr {
		t.Error("delete failed")
	}
}

func TestMCPAuth(t *testing.T) {
	verify := func(_ context.Context, token string) error {
		if token != "secret" {
			return errors.New("bad token")
		}
		return nil
	}
	ts := newMCPServer(t, verify)

	// No token -> unauthorized.
	out := rpc(t, ts, "", "tools/list", map[string]any{})
	if out["error"] == nil {
		t.Errorf("expected unauthorized error, got %v", out)
	}

	// Correct token -> ok.
	out = rpc(t, ts, "secret", "tools/list", map[string]any{})
	if out["error"] != nil {
		t.Errorf("expected success with token, got %v", out)
	}
}
