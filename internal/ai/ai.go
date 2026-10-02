// Package ai provides a thin client for OpenAI-compatible chat completion
// APIs. It is intentionally dependency-free: any provider exposing
// POST {base}/chat/completions works (OpenAI, DeepSeek, Ollama, vLLM, ...).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

// Message is a single chat message.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Client talks to an OpenAI-compatible endpoint.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// New creates a client. An empty baseURL means AI is disabled.
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Enabled reports whether AI features are configured.
func (c *Client) Enabled() bool { return c != nil && c.baseURL != "" }

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat sends the conversation and returns the assistant's reply.
func (c *Client) Chat(ctx context.Context, messages []Message, temperature float64) (string, error) {
	if !c.Enabled() {
		return "", core.Invalidf("AI is not configured")
	}
	payload, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("invalid ai response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("ai provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("ai provider returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
