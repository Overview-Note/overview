package service

import (
	"context"
	"strings"
	"sync"

	"github.com/overview-app/overview/internal/ai"
	"github.com/overview-app/overview/internal/core"
)

// AIConfig is the effective AI configuration (runtime overrides + defaults).
type AIConfig struct {
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
}

// AIConfigStore persists AI configuration overrides.
type AIConfigStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

const (
	settingAIBaseURL = "ai_base_url"
	settingAIAPIKey  = "ai_api_key"
	settingAIModel   = "ai_model"
)

// AIService wraps the LLM client with note-oriented operations. Configuration
// can come from environment defaults or be overridden at runtime via settings.
type AIService struct {
	mu       sync.RWMutex
	store    AIConfigStore
	defaults AIConfig
	current  AIConfig
}

// NewAI constructs an AIService. store may be nil to disable runtime overrides.
func NewAI(store AIConfigStore, defaults AIConfig) *AIService {
	s := &AIService{store: store, defaults: defaults, current: defaults}
	return s
}

// Load refreshes the effective config from the settings store.
func (s *AIService) Load(ctx context.Context) {
	if s.store == nil {
		return
	}
	cfg := s.defaults
	if v, err := s.store.GetSetting(ctx, settingAIBaseURL); err == nil && v != "" {
		cfg.BaseURL = v
	}
	if v, err := s.store.GetSetting(ctx, settingAIAPIKey); err == nil && v != "" {
		cfg.APIKey = v
	}
	if v, err := s.store.GetSetting(ctx, settingAIModel); err == nil && v != "" {
		cfg.Model = v
	}
	s.mu.Lock()
	s.current = cfg
	s.mu.Unlock()
}

func (s *AIService) config() AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *AIService) client() *ai.Client {
	cfg := s.config()
	return ai.New(cfg.BaseURL, cfg.APIKey, cfg.Model)
}

// Enabled reports whether AI features are available.
func (s *AIService) Enabled() bool { return s.client().Enabled() }

// Model returns the configured model name.
func (s *AIService) Model() string {
	cfg := s.config()
	if cfg.Model == "" {
		return "gpt-4o-mini"
	}
	return cfg.Model
}

// Config returns the effective configuration, with the API key masked for
// safe exposure over the API.
func (s *AIService) Config() (baseURL, model string, hasKey bool) {
	cfg := s.config()
	return cfg.BaseURL, s.Model(), cfg.APIKey != ""
}

// SaveConfig persists AI configuration overrides and reloads.
func (s *AIService) SaveConfig(ctx context.Context, cfg AIConfig) error {
	if s.store == nil {
		return core.Forbiddenf("runtime AI configuration is not available")
	}
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)

	if err := s.store.SetSetting(ctx, settingAIBaseURL, cfg.BaseURL); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingAIModel, cfg.Model); err != nil {
		return err
	}
	// Only overwrite the key when a new one is provided.
	if cfg.APIKey != "" {
		if err := s.store.SetSetting(ctx, settingAIAPIKey, cfg.APIKey); err != nil {
			return err
		}
	}
	s.Load(ctx)
	return nil
}

// Chat sends a free-form conversation and returns the reply.
func (s *AIService) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	if len(messages) == 0 {
		return "", core.Invalidf("messages are required")
	}
	return s.client().Chat(ctx, messages, 0.3)
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
	return s.client().Chat(ctx, []ai.Message{
		{Role: "system", Content: organizeSystem},
		{Role: "user", Content: content},
	}, 0.2)
}

// Complete continues a note body.
func (s *AIService) Complete(ctx context.Context, content string) (string, error) {
	if content == "" {
		return "", core.Invalidf("content is required")
	}
	return s.client().Chat(ctx, []ai.Message{
		{Role: "system", Content: completeSystem},
		{Role: "user", Content: content},
	}, 0.4)
}
