package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Overview-Note/overview/internal/ai"
	"github.com/Overview-Note/overview/internal/service"
)

type memSettingStore struct {
	values map[string]string
}

func (s *memSettingStore) GetSetting(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *memSettingStore) SetSetting(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func strPtr(v string) *string { return &v }

// toolRejectingServer returns 400 with a body that looks like a tool-calling
// rejection, driving the probe to "unsupported".
func toolRejectingServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"this model does not support tools"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestToolCallingProbeResetsAfterAgentSettingsSave(t *testing.T) {
	srv := toolRejectingServer(t)
	store := &memSettingStore{values: map[string]string{}}
	svc := service.NewAI(store, service.AIConfig{BaseURL: srv.URL, Model: "test-model"})
	ctx := context.Background()

	if _, err := svc.ChatTools(ctx, nil, nil, 0); !errors.Is(err, ai.ErrToolsUnsupported) {
		t.Fatalf("ChatTools error = %v, want ErrToolsUnsupported", err)
	}
	if got := svc.ToolCallingStatus(); got != "false" {
		t.Fatalf("ToolCallingStatus() = %q, want %q", got, "false")
	}

	if err := svc.SaveAgentSettings(ctx, service.AgentSettingsPatch{
		ToolCalling: strPtr(service.ToolCallingAuto),
	}); err != nil {
		t.Fatalf("SaveAgentSettings: %v", err)
	}
	if got := svc.ToolCallingStatus(); got != "unknown" {
		t.Fatalf("ToolCallingStatus() after save = %q, want %q", got, "unknown")
	}
}

func TestToolCallingProbeResetsAfterConfigSave(t *testing.T) {
	srv := toolRejectingServer(t)
	store := &memSettingStore{values: map[string]string{}}
	svc := service.NewAI(store, service.AIConfig{BaseURL: srv.URL, Model: "test-model"})
	ctx := context.Background()

	if _, err := svc.ChatTools(ctx, nil, nil, 0); !errors.Is(err, ai.ErrToolsUnsupported) {
		t.Fatalf("ChatTools error = %v, want ErrToolsUnsupported", err)
	}
	if got := svc.ToolCallingStatus(); got != "false" {
		t.Fatalf("ToolCallingStatus() = %q, want %q", got, "false")
	}

	if err := svc.SaveConfig(ctx, service.AIConfig{BaseURL: srv.URL, Model: "other-model"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if got := svc.ToolCallingStatus(); got != "unknown" {
		t.Fatalf("ToolCallingStatus() after config save = %q, want %q", got, "unknown")
	}
}

func TestToolCallingStatusRespectsOffMode(t *testing.T) {
	store := &memSettingStore{values: map[string]string{}}
	svc := service.NewAI(store, service.AIConfig{})
	ctx := context.Background()

	if err := svc.SaveAgentSettings(ctx, service.AgentSettingsPatch{
		ToolCalling: strPtr(service.ToolCallingOff),
	}); err != nil {
		t.Fatalf("SaveAgentSettings: %v", err)
	}
	if got := svc.ToolCallingStatus(); got != "false" {
		t.Fatalf("ToolCallingStatus() = %q, want %q", got, "false")
	}
}
