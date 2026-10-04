package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/Overview-Note/overview/internal/ai"
	"github.com/Overview-Note/overview/internal/core"
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

	settingAIAgentEnabled  = "ai_agent_enabled"
	settingAIToolCalling   = "ai_tool_calling"
	settingAIConfirmPolicy = "ai_confirm_policy"
	settingAIMaxSteps      = "ai_max_steps"
	settingAIAllowedTools  = "ai_allowed_tools"
)

// Agent operation defaults.
const (
	DefaultMaxSteps      = 8
	maxStepsCeiling      = 32
	DefaultConfirmPolicy = "dangerous"
	ToolCallingAuto      = "auto"
	ToolCallingOff       = "off"
)

// AgentSettings is the effective agent configuration.
type AgentSettings struct {
	Enabled       bool
	ToolCalling   string
	ConfirmPolicy string
	MaxSteps      int
	AllowedTools  []string
}

// defaultAgentSettings returns the configuration used when nothing is stored.
func defaultAgentSettings() AgentSettings {
	return AgentSettings{
		Enabled:       true,
		ToolCalling:   ToolCallingAuto,
		ConfirmPolicy: DefaultConfirmPolicy,
		MaxSteps:      DefaultMaxSteps,
	}
}

// AgentSettingsPatch is a partial update; nil fields are left unchanged.
type AgentSettingsPatch struct {
	Enabled       *bool
	ToolCalling   *string
	ConfirmPolicy *string
	MaxSteps      *int
	AllowedTools  *[]string
}

// AIService wraps the LLM client with note-oriented operations. Configuration
// can come from environment defaults or be overridden at runtime via settings.
type AIService struct {
	mu       sync.RWMutex
	store    AIConfigStore
	defaults AIConfig
	current  AIConfig

	agent                AgentSettings
	toolCallingKnown     bool
	toolCallingSupported bool
}

// NewAI constructs an AIService. store may be nil to disable runtime overrides.
func NewAI(store AIConfigStore, defaults AIConfig) *AIService {
	s := &AIService{store: store, defaults: defaults, current: defaults, agent: defaultAgentSettings()}
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
	agent := defaultAgentSettings()
	if v, err := s.store.GetSetting(ctx, settingAIAgentEnabled); err == nil && v != "" {
		agent.Enabled = v == "true"
	}
	if v, err := s.store.GetSetting(ctx, settingAIToolCalling); err == nil && v != "" {
		agent.ToolCalling = v
	}
	if v, err := s.store.GetSetting(ctx, settingAIConfirmPolicy); err == nil && v != "" {
		agent.ConfirmPolicy = v
	}
	if v, err := s.store.GetSetting(ctx, settingAIMaxSteps); err == nil && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			agent.MaxSteps = n
		}
	}
	if v, err := s.store.GetSetting(ctx, settingAIAllowedTools); err == nil && v != "" {
		agent.AllowedTools = splitList(v)
	}
	s.mu.Lock()
	s.current = cfg
	s.agent = agent
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
	s.resetToolCallingProbe()
	return nil
}

// AgentConfig returns the effective agent settings.
func (s *AIService) AgentConfig() AgentSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agent
}

// AgentEnabled reports whether the in-app agent is enabled.
func (s *AIService) AgentEnabled() bool { return s.AgentConfig().Enabled }

// ToolCallingMode reports "auto" or "off".
func (s *AIService) ToolCallingMode() string { return s.AgentConfig().ToolCalling }

// ConfirmPolicy reports the confirmation policy ("dangerous", "all" or "none").
func (s *AIService) ConfirmPolicy() string { return s.AgentConfig().ConfirmPolicy }

// MaxSteps reports the agent tool-loop ceiling.
func (s *AIService) MaxSteps() int { return s.AgentConfig().MaxSteps }

// AllowedTools returns the optional server-side whitelist of tool names.
func (s *AIService) AllowedTools() []string { return s.AgentConfig().AllowedTools }

// ToolCalling reports whether the provider is known to support tool calling.
func (s *AIService) ToolCalling() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toolCallingKnown && s.toolCallingSupported
}

// ToolCallingStatus renders the tri-state capability as reported to clients:
// "off" when disabled, "true"/"false" once probed, "unknown" before that.
func (s *AIService) ToolCallingStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.agent.ToolCalling == ToolCallingOff {
		return "false"
	}
	if !s.toolCallingKnown {
		return "unknown"
	}
	if s.toolCallingSupported {
		return "true"
	}
	return "false"
}

// resetToolCallingProbe marks the provider's tool-calling support as unknown so
// the next agent request re-probes it. Called whenever configuration that may
// change the provider's capabilities (baseUrl, model or toolCalling mode) is
// saved, so a stale "unsupported" result self-heals without a restart.
func (s *AIService) resetToolCallingProbe() {
	s.mu.Lock()
	s.toolCallingKnown = false
	s.toolCallingSupported = false
	s.mu.Unlock()
}

// SaveAgentSettings applies a partial update to the agent configuration.
func (s *AIService) SaveAgentSettings(ctx context.Context, p AgentSettingsPatch) error {
	if s.store == nil {
		return core.Forbiddenf("runtime AI configuration is not available")
	}
	if p.ToolCalling != nil {
		v := strings.TrimSpace(*p.ToolCalling)
		if v != ToolCallingAuto && v != ToolCallingOff {
			return core.Invalidf("toolCalling must be 'auto' or 'off'")
		}
		if err := s.store.SetSetting(ctx, settingAIToolCalling, v); err != nil {
			return err
		}
	}
	if p.ConfirmPolicy != nil {
		v := strings.TrimSpace(*p.ConfirmPolicy)
		if v != "dangerous" && v != "all" && v != "none" {
			return core.Invalidf("confirmPolicy must be 'dangerous', 'all' or 'none'")
		}
		if err := s.store.SetSetting(ctx, settingAIConfirmPolicy, v); err != nil {
			return err
		}
	}
	if p.MaxSteps != nil {
		n := *p.MaxSteps
		if n < 1 || n > maxStepsCeiling {
			return core.Invalidf("maxSteps must be between 1 and %d", maxStepsCeiling)
		}
		if err := s.store.SetSetting(ctx, settingAIMaxSteps, strconv.Itoa(n)); err != nil {
			return err
		}
	}
	if p.Enabled != nil {
		if err := s.store.SetSetting(ctx, settingAIAgentEnabled, strconv.FormatBool(*p.Enabled)); err != nil {
			return err
		}
	}
	if p.AllowedTools != nil {
		if err := s.store.SetSetting(ctx, settingAIAllowedTools, strings.Join(cleanList(*p.AllowedTools), ",")); err != nil {
			return err
		}
	}
	s.Load(ctx)
	s.resetToolCallingProbe()
	return nil
}

// ChatTools runs a tool-enabled completion and caches the provider's tool
// support when it explicitly rejects the request.
func (s *AIService) ChatTools(ctx context.Context, messages []ai.AgentMessage, offered []ai.Tool, temperature float64) (ai.ChatResult, error) {
	res, err := s.client().ChatTools(ctx, messages, offered, temperature)
	if errors.Is(err, ai.ErrToolsUnsupported) {
		s.mu.Lock()
		s.toolCallingKnown = true
		s.toolCallingSupported = false
		s.mu.Unlock()
		return ai.ChatResult{}, err
	}
	if err == nil {
		s.mu.Lock()
		s.toolCallingKnown = true
		s.toolCallingSupported = true
		s.mu.Unlock()
	}
	return res, err
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
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
