package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Overview-Note/overview/internal/ai"
)

func TestDisabled(t *testing.T) {
	c := ai.New("", "", "")
	if c.Enabled() {
		t.Fatal("empty base URL should be disabled")
	}
	if _, err := c.Chat(context.Background(), nil, 0); err == nil {
		t.Fatal("expected error when disabled")
	}
}

func TestChat(t *testing.T) {
	var gotAuth, gotPath string
	var gotReq struct {
		Model    string       `json:"model"`
		Messages []ai.Message `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi there"}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "secret", "test-model")
	if !c.Enabled() || c.Model() != "test-model" {
		t.Fatalf("unexpected client state")
	}
	reply, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "hi there" {
		t.Fatalf("reply = %q", reply)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotReq.Model != "test-model" || len(gotReq.Messages) != 1 {
		t.Errorf("request payload = %+v", gotReq)
	}
}

func TestChatProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "", "m")
	if _, err := c.Chat(context.Background(), nil, 0); err == nil {
		t.Fatal("expected provider error")
	}
}
