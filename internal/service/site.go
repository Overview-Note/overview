package service

import (
	"context"
	"strings"
	"sync"

	"github.com/Overview-Note/overview/internal/core"
)

// SiteConfig is the login-page presentation configuration. Every field is
// non-sensitive and may be exposed on the public auth state endpoint.
type SiteConfig struct {
	RegistrationEnabled bool   `json:"registrationEnabled"`
	LoginHint           string `json:"loginHint"`
	LoginICP            string `json:"loginIcp"`
	LoginLinkURL        string `json:"loginLinkUrl"`
	LoginLinkText       string `json:"loginLinkText"`
}

const (
	settingRegistrationEnabled = "registration_enabled"
	settingLoginHint           = "login_hint"
	settingLoginICP            = "login_icp"
	settingLoginLinkURL        = "login_link_url"
	settingLoginLinkText       = "login_link_text"
)

// SiteService stores and exposes the public site settings shown on the login
// and registration pages.
type SiteService struct {
	mu      sync.RWMutex
	store   settingsStore
	current SiteConfig
}

// NewSite constructs a SiteService. store may be nil to disable persistence.
func NewSite(store settingsStore) *SiteService {
	return &SiteService{store: store}
}

// Load refreshes the effective config from the settings store.
func (s *SiteService) Load(ctx context.Context) {
	if s.store == nil {
		return
	}
	cfg := SiteConfig{}
	if v, err := s.store.GetSetting(ctx, settingRegistrationEnabled); err == nil {
		cfg.RegistrationEnabled = v == "on"
	}
	if v, err := s.store.GetSetting(ctx, settingLoginHint); err == nil {
		cfg.LoginHint = v
	}
	if v, err := s.store.GetSetting(ctx, settingLoginICP); err == nil {
		cfg.LoginICP = v
	}
	if v, err := s.store.GetSetting(ctx, settingLoginLinkURL); err == nil {
		cfg.LoginLinkURL = v
	}
	if v, err := s.store.GetSetting(ctx, settingLoginLinkText); err == nil {
		cfg.LoginLinkText = v
	}
	s.mu.Lock()
	s.current = cfg
	s.mu.Unlock()
}

// Config returns the effective site configuration.
func (s *SiteService) Config() SiteConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// RegistrationEnabled reports whether self-registration is currently allowed.
func (s *SiteService) RegistrationEnabled() bool {
	return s.Config().RegistrationEnabled
}

// SaveConfig persists the site configuration and reloads. Strings are trimmed;
// the URL is stored as-is and is validated by the client before rendering.
func (s *SiteService) SaveConfig(ctx context.Context, cfg SiteConfig) error {
	if s.store == nil {
		return core.Forbiddenf("runtime site configuration is not available")
	}
	cfg.LoginHint = strings.TrimSpace(cfg.LoginHint)
	cfg.LoginICP = strings.TrimSpace(cfg.LoginICP)
	cfg.LoginLinkURL = strings.TrimSpace(cfg.LoginLinkURL)
	cfg.LoginLinkText = strings.TrimSpace(cfg.LoginLinkText)

	enabled := "off"
	if cfg.RegistrationEnabled {
		enabled = "on"
	}
	if err := s.store.SetSetting(ctx, settingRegistrationEnabled, enabled); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingLoginHint, cfg.LoginHint); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingLoginICP, cfg.LoginICP); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingLoginLinkURL, cfg.LoginLinkURL); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingLoginLinkText, cfg.LoginLinkText); err != nil {
		return err
	}
	s.Load(ctx)
	return nil
}
