// Package ai provides a thin client for OpenAI-compatible chat completion
// APIs. It is intentionally dependency-free: any provider exposing
// POST {base}/chat/completions works (OpenAI, DeepSeek, Ollama, vLLM, ...).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

// ErrToolsUnsupported is returned by ChatTools when the provider explicitly
// rejects a request because it does not support tool/function calling.
var ErrToolsUnsupported = errors.New("ai provider does not support tool calling")

// Message is a single chat message.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// ToolCall is a function call requested by the assistant.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool is a callable tool offered to the model.
type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

// AgentMessage is a chat message that may carry tool calls or tool results.
type AgentMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ChatResult is a parsed non-streaming completion.
type ChatResult struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
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
	Model       string  `json:"model"`
	Messages    any     `json:"messages"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream"`
	Tools       []Tool  `json:"tools,omitempty"`
	ToolChoice  string  `json:"tool_choice,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      AgentMessage `json:"message"`
		FinishReason string       `json:"finish_reason"`
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

// ChatTools sends a conversation together with a tool list and returns the
// assistant's text plus any tool calls it requested. When the provider rejects
// the request because it does not support tools, it returns
// ErrToolsUnsupported so the caller can cache that fact.
func (c *Client) ChatTools(ctx context.Context, messages []AgentMessage, offered []Tool, temperature float64) (ChatResult, error) {
	if !c.Enabled() {
		return ChatResult{}, core.Invalidf("AI is not configured")
	}
	payload, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		Tools:       offered,
		ToolChoice:  "auto",
	})
	if err != nil {
		return ChatResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return ChatResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ChatResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return ChatResult{}, err
	}
	if resp.StatusCode >= 300 {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && looksLikeToolRejection(body) {
			return ChatResult{}, ErrToolsUnsupported
		}
		return ChatResult{}, fmt.Errorf("ai provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ChatResult{}, fmt.Errorf("invalid ai response: %w", err)
	}
	if parsed.Error != nil {
		return ChatResult{}, fmt.Errorf("ai provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("ai provider returned no choices")
	}
	choice := parsed.Choices[0]
	return ChatResult{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
	}, nil
}

// looksLikeToolRejection reports whether a 4xx body indicates the provider does
// not implement tool/function calling.
func looksLikeToolRejection(body []byte) bool {
	msg := strings.ToLower(string(body))
	return strings.Contains(msg, "tool") || strings.Contains(msg, "function")
}
