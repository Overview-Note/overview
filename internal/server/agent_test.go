package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Overview-Note/overview/internal/agent"
	"github.com/Overview-Note/overview/internal/ai"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

type serverFakeLLM struct {
	mu      sync.Mutex
	results []ai.ChatResult
	i       int
}

func (f *serverFakeLLM) ChatTools(_ context.Context, _ []ai.AgentMessage, _ []ai.Tool, _ float64) (ai.ChatResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.i >= len(f.results) {
		return ai.ChatResult{Content: "done", FinishReason: "stop"}, nil
	}
	res := f.results[f.i]
	f.i++
	return res, nil
}

func serverToolCall(id, name, args string) ai.ChatResult {
	var tc ai.ToolCall
	tc.ID = id
	tc.Type = "function"
	tc.Function.Name = name
	tc.Function.Arguments = args
	return ai.ChatResult{FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{tc}}
}

func newAgentServer(t *testing.T, llm agent.LLM) *httptest.Server {
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
	svc := service.New(st, ix, st, nil, nil)
	aiSvc := service.NewAI(ix, service.AIConfig{BaseURL: "http://127.0.0.1:1/v1", Model: "m"})
	agt := agent.New(svc, llm, ix)
	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		AI:             aiSvc,
		Agent:          agt,
	}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestAgentRunAndConfirm(t *testing.T) {
	llm := &serverFakeLLM{results: []ai.ChatResult{
		serverToolCall("c1", "notes_write", `{"path":"agent.md","body":"hi","baseVersion":"*"}`),
		{Content: "created", FinishReason: "stop"},
	}}
	ts := newAgentServer(t, llm)

	resp, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent", map[string]any{"input": "create agent.md"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %v", resp.StatusCode, body)
	}
	if body["status"] != "done" || body["runId"] == "" {
		t.Fatalf("body = %v", body)
	}
	steps, _ := body["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps = %v", body["steps"])
	}

	// The note really exists.
	getResp, _ := doJSON(t, http.MethodGet, ts.URL+"/api/v1/note?path=agent.md", nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("note not created: %d", getResp.StatusCode)
	}
}

func TestAgentConfirmDangerous(t *testing.T) {
	// Reject: the note survives.
	llm := &serverFakeLLM{results: []ai.ChatResult{
		serverToolCall("c1", "notes_delete", `{"path":"victim.md"}`),
		{Content: "done", FinishReason: "stop"},
	}}
	ts := newAgentServer(t, llm)
	if resp, b := doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "victim.md", "body": "data", "baseVersion": "*",
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("seed note: %d %v", resp.StatusCode, b)
	}

	resp, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent", map[string]any{"input": "delete victim"})
	if resp.StatusCode != http.StatusOK || body["status"] != "needs_confirmation" {
		t.Fatalf("agent = %d %v", resp.StatusCode, body)
	}
	pending, _ := body["pending"].(map[string]any)
	if pending == nil || pending["name"] != "notes_delete" || pending["preview"] == "" {
		t.Fatalf("pending = %v", pending)
	}

	rejResp, rejBody := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent/confirm", map[string]any{
		"runId":      body["runId"],
		"toolCallId": pending["toolCallId"],
		"decision":   "reject",
	})
	if rejResp.StatusCode != http.StatusOK || rejBody["status"] != "done" {
		t.Fatalf("reject = %d %v", rejResp.StatusCode, rejBody)
	}
	if r, _ := doJSON(t, http.MethodGet, ts.URL+"/api/v1/note?path=victim.md", nil); r.StatusCode != http.StatusOK {
		t.Fatal("note should survive rejection")
	}
}

func TestAgentConfirmApprove(t *testing.T) {
	llm := &serverFakeLLM{results: []ai.ChatResult{
		serverToolCall("c1", "notes_delete", `{"path":"victim.md"}`),
		{Content: "done", FinishReason: "stop"},
	}}
	ts := newAgentServer(t, llm)
	doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "victim.md", "body": "data", "baseVersion": "*",
	})
	_, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent", map[string]any{"input": "delete"})
	pending, _ := body["pending"].(map[string]any)

	resp, appBody := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent/confirm", map[string]any{
		"runId":      body["runId"],
		"toolCallId": pending["toolCallId"],
		"decision":   "approve",
	})
	if resp.StatusCode != http.StatusOK || appBody["status"] != "done" {
		t.Fatalf("approve = %d %v", resp.StatusCode, appBody)
	}
	if r, _ := doJSON(t, http.MethodGet, ts.URL+"/api/v1/note?path=victim.md", nil); r.StatusCode == http.StatusOK {
		t.Fatal("note should be deleted after approval")
	}
}

func TestAgentStop(t *testing.T) {
	llm := &serverFakeLLM{results: []ai.ChatResult{
		serverToolCall("c1", "notes_delete", `{"path":"x.md"}`),
	}}
	ts := newAgentServer(t, llm)
	doJSON(t, http.MethodPut, ts.URL+"/api/v1/note", map[string]string{
		"path": "x.md", "body": "data", "baseVersion": "*",
	})
	_, body := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent", map[string]any{"input": "delete"})
	runID, _ := body["runId"].(string)

	resp, stopBody := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent/stop", map[string]any{"runId": runID})
	if resp.StatusCode != http.StatusOK || stopBody["status"] != "stopped" {
		t.Fatalf("stop = %d %v", resp.StatusCode, stopBody)
	}

	pending, _ := body["pending"].(map[string]any)
	confResp, _ := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent/confirm", map[string]any{
		"runId":      runID,
		"toolCallId": pending["toolCallId"],
		"decision":   "approve",
	})
	if confResp.StatusCode != http.StatusNotFound {
		t.Fatalf("confirm after stop = %d, want 404", confResp.StatusCode)
	}
}

func TestAIStatusHasAgentFields(t *testing.T) {
	llm := &serverFakeLLM{}
	ts := newAgentServer(t, llm)
	resp, body := doJSON(t, http.MethodGet, ts.URL+"/api/v1/ai/status", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if body["toolCalling"] != "unknown" {
		t.Errorf("toolCalling = %v", body["toolCalling"])
	}
	if body["agentEnabled"] != true {
		t.Errorf("agentEnabled = %v", body["agentEnabled"])
	}
	if body["confirmPolicy"] != "dangerous" {
		t.Errorf("confirmPolicy = %v", body["confirmPolicy"])
	}
	if _, ok := body["maxSteps"]; !ok {
		t.Errorf("missing maxSteps: %v", body)
	}
}

func TestAISettingsPartialUpdate(t *testing.T) {
	ts, client := newAuthServer(t)
	authJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/setup",
		map[string]string{"username": "admin", "password": "secret1"}, nil)

	// Set base config.
	var saved map[string]any
	if code := authJSON(t, client, http.MethodPut, ts.URL+"/api/v1/settings/ai",
		map[string]any{"baseUrl": "https://api.example.com/v1", "model": "m"}, &saved); code != http.StatusOK {
		t.Fatalf("save base = %d %v", code, saved)
	}
	// Partial update of agent fields must not clear the base URL.
	if code := authJSON(t, client, http.MethodPut, ts.URL+"/api/v1/settings/ai",
		map[string]any{"confirmPolicy": "all", "maxSteps": 4, "agentEnabled": false}, &saved); code != http.StatusOK {
		t.Fatalf("partial save = %d %v", code, saved)
	}
	if saved["baseUrl"] != "https://api.example.com/v1" {
		t.Errorf("baseUrl cleared by partial update: %v", saved["baseUrl"])
	}
	if saved["confirmPolicy"] != "all" {
		t.Errorf("confirmPolicy = %v", saved["confirmPolicy"])
	}
	if saved["agentEnabled"] != false {
		t.Errorf("agentEnabled = %v", saved["agentEnabled"])
	}
	if n, _ := saved["maxSteps"].(float64); int(n) != 4 {
		t.Errorf("maxSteps = %v", saved["maxSteps"])
	}

	// Read back.
	var cfg map[string]any
	authJSON(t, client, http.MethodGet, ts.URL+"/api/v1/settings/ai", nil, &cfg)
	if cfg["baseUrl"] != "https://api.example.com/v1" || cfg["confirmPolicy"] != "all" {
		t.Errorf("read back = %v", cfg)
	}
}

func TestAgentEmptyInput(t *testing.T) {
	llm := &serverFakeLLM{}
	ts := newAgentServer(t, llm)
	resp, _ := doJSON(t, http.MethodPost, ts.URL+"/api/v1/ai/agent", map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty input status = %d, want 400", resp.StatusCode)
	}
}
