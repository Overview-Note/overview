package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Overview-Note/overview/internal/ai"
)

func TestChatToolsParsesToolCalls(t *testing.T) {
	var got struct {
		Tools      []map[string]any `json:"tools"`
		ToolChoice string           `json:"tool_choice"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"notes_read","arguments":"{\"path\":\"a.md\"}"}}]}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "", "m")
	var tool ai.Tool
	tool.Type = "function"
	tool.Function.Name = "notes_read"
	tool.Function.Parameters = map[string]any{"type": "object"}

	res, err := c.ChatTools(context.Background(),
		[]ai.AgentMessage{{Role: "user", Content: "read a.md"}}, []ai.Tool{tool}, 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if res.FinishReason != "tool_calls" {
		t.Errorf("finish reason = %q", res.FinishReason)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(res.ToolCalls))
	}
	tc := res.ToolCalls[0]
	if tc.ID != "call_1" || tc.Function.Name != "notes_read" || tc.Function.Arguments != `{"path":"a.md"}` {
		t.Errorf("tool call = %+v", tc)
	}
	if got.ToolChoice != "auto" || len(got.Tools) != 1 {
		t.Errorf("request tools = %+v choice=%q", got.Tools, got.ToolChoice)
	}
}

func TestChatToolsUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"this model does not support tools"}}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "", "m")
	_, err := c.ChatTools(context.Background(), nil, nil, 0)
	if !errors.Is(err, ai.ErrToolsUnsupported) {
		t.Fatalf("err = %v, want ErrToolsUnsupported", err)
	}
}

func TestChatToolsUnrelatedBadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "", "m")
	_, err := c.ChatTools(context.Background(), nil, nil, 0)
	if err == nil || errors.Is(err, ai.ErrToolsUnsupported) {
		t.Fatalf("err = %v, should be a plain provider error", err)
	}
}

func TestChatToolsFinishReasonWithoutCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "", "m")
	res, err := c.ChatTools(context.Background(), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "done" || len(res.ToolCalls) != 0 || res.FinishReason != "stop" {
		t.Errorf("result = %+v", res)
	}
}
