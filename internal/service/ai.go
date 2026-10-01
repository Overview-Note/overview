package service

import (
	"context"

	"github.com/overview-app/overview/internal/ai"
	"github.com/overview-app/overview/internal/core"
)

// AIService wraps the LLM client with note-oriented operations.
type AIService struct {
	client *ai.Client
}

// NewAI constructs an AIService.
func NewAI(client *ai.Client) *AIService {
	return &AIService{client: client}
}

// Enabled reports whether AI features are available.
func (s *AIService) Enabled() bool { return s.client.Enabled() }

// Model returns the configured model name.
func (s *AIService) Model() string { return s.client.Model() }

// Chat sends a free-form conversation and returns the reply.
func (s *AIService) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	if len(messages) == 0 {
		return "", core.Invalidf("messages are required")
	}
	return s.client.Chat(ctx, messages, 0.3)
}

const organizeSystem = `You are an assistant embedded in a Markdown knowledge base.
Rewrite and organize the provided note. Keep it in valid Markdown.
Improve structure, headings and clarity, fix obvious typos, but do NOT invent facts.
Return only the rewritten Markdown, with no surrounding explanation or code fence.`

const completeSystem = `You are an assistant embedded in a Markdown knowledge base.
Continue the provided Markdown note naturally, matching its tone and style.
Return only the continuation text (valid Markdown), with no repetition of the input and no explanation.`

// Organize rewrites a note body into a cleaner structure.
func (s *AIService) Organize(ctx context.Context, content string) (string, error) {
	if content == "" {
		return "", core.Invalidf("content is required")
	}
	return s.client.Chat(ctx, []ai.Message{
		{Role: "system", Content: organizeSystem},
		{Role: "user", Content: content},
	}, 0.2)
}

// Complete continues a note body.
func (s *AIService) Complete(ctx context.Context, content string) (string, error) {
	if content == "" {
		return "", core.Invalidf("content is required")
	}
	return s.client.Chat(ctx, []ai.Message{
		{Role: "system", Content: completeSystem},
		{Role: "user", Content: content},
	}, 0.4)
}
