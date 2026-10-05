package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/s3store"
)

const (
	settingS3Endpoint   = "s3_endpoint"
	settingS3Region     = "s3_region"
	settingS3AccessKey  = "s3_access_key"
	settingS3SecretKey  = "s3_secret_key"
	settingS3Bucket     = "s3_bucket"
	settingS3UseSSL     = "s3_use_ssl"
	settingS3PublicURL  = "s3_public_url"
	settingS3Configured = "s3_configured"
)

// DefaultS3Region is assumed when no region is configured.
const DefaultS3Region = "us-east-1"

// SwitchableAssetStore is a core.AssetStore that delegates to a mutable current
// backend. It lets the asset store be swapped between the local filesystem and
// an S3-compatible bucket at runtime without restarting the process.
type SwitchableAssetStore struct {
	mu      sync.RWMutex
	current core.AssetStore
}

// NewSwitchableAssetStore wraps an initial asset store.
func NewSwitchableAssetStore(initial core.AssetStore) *SwitchableAssetStore {
	return &SwitchableAssetStore{current: initial}
}

// Set replaces the active backend. A nil store is ignored so a failed
// configuration can never leave the wrapper without a working backend.
func (a *SwitchableAssetStore) Set(store core.AssetStore) {
	if store == nil {
		return
	}
	a.mu.Lock()
	a.current = store
	a.mu.Unlock()
}

// Get returns the active backend.
func (a *SwitchableAssetStore) Get() core.AssetStore {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

// Save stores r under name using the active backend.
func (a *SwitchableAssetStore) Save(ctx context.Context, name string, r io.Reader) (core.Asset, error) {
	return a.Get().Save(ctx, name, r)
}

// Open returns a reader for the asset at rel using the active backend.
func (a *SwitchableAssetStore) Open(ctx context.Context, rel string) (io.ReadSeekCloser, error) {
	return a.Get().Open(ctx, rel)
}

// ListAssets returns metadata for every stored asset using the active backend.
func (a *SwitchableAssetStore) ListAssets(ctx context.Context) ([]core.Asset, error) {
	return a.Get().ListAssets(ctx)
}

// DeleteAsset removes the asset at rel using the active backend.
func (a *SwitchableAssetStore) DeleteAsset(ctx context.Context, rel string) error {
	return a.Get().DeleteAsset(ctx, rel)
}

// Restore delegates to the active backend when it supports explicit-path
// restoration, and reports ErrNotSupported otherwise (an S3 bucket cannot
// restore an archive path directly). Archive import treats that as a skip.
func (a *SwitchableAssetStore) Restore(ctx context.Context, rel string, r io.Reader) error {
	restorer, ok := a.Get().(core.AssetRestorer)
	if !ok {
		return fmt.Errorf("%w: asset restore is not supported by the active backend", core.ErrNotSupported)
	}
	return restorer.Restore(ctx, rel, r)
}

// StorageConfig is the S3-compatible asset backend configuration.
type StorageConfig struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Bucket    string `json:"bucket"`
	UseSSL    bool   `json:"useSSL"`
	PublicURL string `json:"publicURL"`
}

// StoreFactory builds an asset store from a configuration. It is a seam so
// tests can substitute the S3 constructor without a real object store.
type StoreFactory func(ctx context.Context, cfg StorageConfig) (core.AssetStore, error)

// defaultStoreFactory builds an S3-backed asset store and ensures the bucket
// exists.
func defaultStoreFactory(ctx context.Context, cfg StorageConfig) (core.AssetStore, error) {
	return s3store.New(ctx, s3store.Options{
		Endpoint:  cfg.Endpoint,
		Region:    cfg.Region,
		AccessKey: cfg.AccessKey,
		SecretKey: cfg.SecretKey,
		Bucket:    cfg.Bucket,
		UseSSL:    cfg.UseSSL,
		PublicURL: cfg.PublicURL,
	})
}

// StorageService stores the runtime asset-backend configuration, persists it
// through the settings store and swaps the active asset store accordingly.
type StorageService struct {
	mu       sync.RWMutex
	store    settingsStore
	local    core.AssetStore
	target   *SwitchableAssetStore
	factory  StoreFactory
	defaults StorageConfig
	current  StorageConfig
}

// NewStorage constructs a StorageService. local is the fallback store used when
// no bucket is configured; target receives the active store after each change.
func NewStorage(store settingsStore, local core.AssetStore, target *SwitchableAssetStore) *StorageService {
	def := StorageConfig{Region: DefaultS3Region, UseSSL: true}
	return &StorageService{
		store:    store,
		local:    local,
		target:   target,
		factory:  defaultStoreFactory,
		defaults: def,
		current:  def,
	}
}

// SetDefaults installs the environment-derived configuration used until a
// runtime configuration is saved. It must be called before Load.
func (s *StorageService) SetDefaults(cfg StorageConfig) {
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = DefaultS3Region
	}
	s.mu.Lock()
	s.defaults = cfg
	s.mu.Unlock()
}

// SetStoreFactory overrides how asset stores are constructed. It is intended
// for tests.
func (s *StorageService) SetStoreFactory(f StoreFactory) {
	s.mu.Lock()
	s.factory = f
	s.mu.Unlock()
}

func (s *StorageService) config() StorageConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Config returns the effective configuration with the secret key masked.
// The second result reports whether a secret key is stored.
func (s *StorageService) Config() (StorageConfig, bool) {
	cfg := s.config()
	hasSecret := cfg.SecretKey != ""
	cfg.SecretKey = ""
	return cfg, hasSecret
}

// Enabled reports whether an S3 bucket is configured.
func (s *StorageService) Enabled() bool { return s.config().Bucket != "" }

// Load refreshes the effective configuration. When a runtime configuration was
// previously saved it is applied (replacing the environment default). A build
// failure is returned but leaves the active store untouched.
func (s *StorageService) Load(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	s.mu.RLock()
	def := s.defaults
	s.mu.RUnlock()

	cfg := def
	configured := false
	if v, err := s.store.GetSetting(ctx, settingS3Configured); err == nil && v == "true" {
		configured = true
		cfg = StorageConfig{Region: DefaultS3Region, UseSSL: true}
		if v, err := s.store.GetSetting(ctx, settingS3Endpoint); err == nil {
			cfg.Endpoint = v
		}
		if v, err := s.store.GetSetting(ctx, settingS3Region); err == nil && v != "" {
			cfg.Region = v
		}
		if v, err := s.store.GetSetting(ctx, settingS3AccessKey); err == nil {
			cfg.AccessKey = v
		}
		if v, err := s.store.GetSetting(ctx, settingS3SecretKey); err == nil {
			cfg.SecretKey = v
		}
		if v, err := s.store.GetSetting(ctx, settingS3Bucket); err == nil {
			cfg.Bucket = v
		}
		if v, err := s.store.GetSetting(ctx, settingS3UseSSL); err == nil && v != "" {
			cfg.UseSSL = v == "true"
		}
		if v, err := s.store.GetSetting(ctx, settingS3PublicURL); err == nil {
			cfg.PublicURL = v
		}
	}

	s.mu.Lock()
	s.current = cfg
	s.mu.Unlock()

	if !configured {
		// The environment default is applied by the caller at startup.
		return nil
	}
	return s.apply(ctx, cfg)
}

// SaveConfig validates and persists a new configuration, then swaps the active
// asset store. The secret key is only overwritten when a non-empty value is
// provided. On a build failure nothing is persisted and the active store is
// left untouched.
func (s *StorageService) SaveConfig(ctx context.Context, cfg StorageConfig) error {
	if s.store == nil {
		return core.Forbiddenf("runtime storage configuration is not available")
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.PublicURL = strings.TrimSpace(cfg.PublicURL)
	if cfg.Region == "" {
		cfg.Region = DefaultS3Region
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = s.config().SecretKey
	}

	var built core.AssetStore
	if cfg.Bucket != "" {
		store, err := s.build(ctx, cfg)
		if err != nil {
			return storageBuildError(cfg, err)
		}
		built = store
	}

	if err := s.store.SetSetting(ctx, settingS3Endpoint, cfg.Endpoint); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingS3Region, cfg.Region); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingS3AccessKey, cfg.AccessKey); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingS3Bucket, cfg.Bucket); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingS3UseSSL, strconv.FormatBool(cfg.UseSSL)); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingS3PublicURL, cfg.PublicURL); err != nil {
		return err
	}
	if cfg.SecretKey != "" {
		if err := s.store.SetSetting(ctx, settingS3SecretKey, cfg.SecretKey); err != nil {
			return err
		}
	}
	if err := s.store.SetSetting(ctx, settingS3Configured, "true"); err != nil {
		return err
	}

	s.mu.Lock()
	s.current = cfg
	s.mu.Unlock()

	if built != nil {
		s.target.Set(built)
	} else {
		s.target.Set(s.local)
	}
	return nil
}

// Test checks that the current configuration can be used to open an asset
// store. It never changes the active backend.
func (s *StorageService) Test(ctx context.Context) error {
	return s.TestConfig(ctx, s.config())
}

// TestConfig checks that cfg can be used to open an asset store without
// persisting it or changing the active backend. An empty secret key reuses the
// stored secret, and an empty region falls back to the default.
func (s *StorageService) TestConfig(ctx context.Context, cfg StorageConfig) error {
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.PublicURL = strings.TrimSpace(cfg.PublicURL)
	if cfg.Region == "" {
		cfg.Region = DefaultS3Region
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = s.config().SecretKey
	}
	if cfg.Bucket == "" {
		return core.Invalidf("bucket is required")
	}
	if _, err := s.build(ctx, cfg); err != nil {
		return storageBuildError(cfg, err)
	}
	return nil
}

// storageBuildError wraps a store construction failure with the endpoint and
// bucket so the caller can diagnose it. The secret key is never included.
func storageBuildError(cfg StorageConfig, err error) error {
	return core.Invalidf("failed to initialize storage at %s/%s: %v", cfg.Endpoint, cfg.Bucket, err)
}

// apply rebuilds and swaps the active store for cfg.
func (s *StorageService) apply(ctx context.Context, cfg StorageConfig) error {
	if cfg.Bucket == "" {
		s.target.Set(s.local)
		return nil
	}
	store, err := s.build(ctx, cfg)
	if err != nil {
		return err
	}
	s.target.Set(store)
	return nil
}

func (s *StorageService) build(ctx context.Context, cfg StorageConfig) (core.AssetStore, error) {
	s.mu.RLock()
	factory := s.factory
	s.mu.RUnlock()
	if factory == nil {
		return nil, core.Invalidf("storage factory is not configured")
	}
	return factory(ctx, cfg)
}
