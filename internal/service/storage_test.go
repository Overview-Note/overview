package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/core"
)

type storageMemStore struct {
	values map[string]string
}

func (s *storageMemStore) GetSetting(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *storageMemStore) SetSetting(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

type fakeAssetStore struct {
	id string
}

func (f *fakeAssetStore) Save(_ context.Context, _ string, _ io.Reader) (core.Asset, error) {
	return core.Asset{Path: f.id}, nil
}

func (f *fakeAssetStore) Open(_ context.Context, _ string) (io.ReadSeekCloser, error) {
	return nil, core.ErrNotFound
}

func (f *fakeAssetStore) ListAssets(_ context.Context) ([]core.Asset, error) {
	return nil, nil
}

func (f *fakeAssetStore) DeleteAsset(_ context.Context, _ string) error { return nil }

type fakeRestoreStore struct {
	fakeAssetStore
	restored string
	body     string
}

func (f *fakeRestoreStore) Restore(_ context.Context, rel string, r io.Reader) error {
	b, _ := io.ReadAll(r)
	f.restored = rel
	f.body = string(b)
	return nil
}

func TestSwitchableAssetStoreRestoreDelegates(t *testing.T) {
	restore := &fakeRestoreStore{fakeAssetStore: fakeAssetStore{id: "s3"}}
	wrapper := NewSwitchableAssetStore(restore)
	if err := wrapper.Restore(context.Background(), "2026/10/pic.png", strings.NewReader("payload")); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restore.restored != "2026/10/pic.png" || restore.body != "payload" {
		t.Errorf("delegated restore = %q/%q", restore.restored, restore.body)
	}

	// A backend without the capability still reports ErrNotSupported.
	bare := NewSwitchableAssetStore(&fakeAssetStore{id: "bare"})
	if err := bare.Restore(context.Background(), "x", strings.NewReader("y")); !errors.Is(err, core.ErrNotSupported) {
		t.Errorf("Restore on bare store = %v, want ErrNotSupported", err)
	}
}

func activeID(s *SwitchableAssetStore) string {
	if f, ok := s.Get().(*fakeAssetStore); ok {
		return f.id
	}
	return ""
}

func TestStorageSaveConfigMasksSecretAndKeepsExisting(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{}}
	local := &fakeAssetStore{id: "local"}
	wrapper := NewSwitchableAssetStore(local)
	svc := NewStorage(mem, local, wrapper)
	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return &fakeAssetStore{id: "s3"}, nil
	})
	ctx := context.Background()

	if err := svc.SaveConfig(ctx, StorageConfig{
		Endpoint: "e", Region: "r", AccessKey: "a", SecretKey: "sec", Bucket: "b", UseSSL: true, PublicURL: "p",
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, has := svc.Config()
	if !has {
		t.Errorf("hasSecret = false, want true")
	}
	if got.SecretKey != "" {
		t.Errorf("Config leaked secret: %q", got.SecretKey)
	}
	if !got.UseSSL || got.Endpoint != "e" || got.Bucket != "b" {
		t.Errorf("config = %+v", got)
	}
	if !svc.Enabled() {
		t.Errorf("Enabled() = false, want true")
	}
	if id := activeID(wrapper); id != "s3" {
		t.Errorf("active store = %q, want s3", id)
	}

	// An empty secret on the next save must not clear the stored one.
	if err := svc.SaveConfig(ctx, StorageConfig{Endpoint: "e2", Bucket: "b2"}); err != nil {
		t.Fatalf("SaveConfig 2: %v", err)
	}
	if svc.config().SecretKey != "sec" {
		t.Errorf("secret overwritten: %q", svc.config().SecretKey)
	}
	if mem.values[settingS3SecretKey] != "sec" {
		t.Errorf("persisted secret = %q, want sec", mem.values[settingS3SecretKey])
	}
}

func TestStorageClearBucketFallsBackToLocal(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{}}
	local := &fakeAssetStore{id: "local"}
	wrapper := NewSwitchableAssetStore(local)
	svc := NewStorage(mem, local, wrapper)
	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return &fakeAssetStore{id: "s3"}, nil
	})
	ctx := context.Background()

	if err := svc.SaveConfig(ctx, StorageConfig{Bucket: "b", SecretKey: "sec"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if id := activeID(wrapper); id != "s3" {
		t.Fatalf("active = %q, want s3", id)
	}

	if err := svc.SaveConfig(ctx, StorageConfig{Bucket: ""}); err != nil {
		t.Fatalf("clear SaveConfig: %v", err)
	}
	if svc.Enabled() {
		t.Errorf("Enabled() = true after clearing bucket")
	}
	if id := activeID(wrapper); id != "local" {
		t.Errorf("active = %q, want local", id)
	}

	// A restart must also resolve to local from the persisted state.
	restarted := NewStorage(mem, local, NewSwitchableAssetStore(local))
	restarted.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return &fakeAssetStore{id: "s3"}, nil
	})
	if err := restarted.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if restarted.Enabled() {
		t.Errorf("restarted Enabled() = true, want false")
	}
}

func TestStorageBuildFailureDoesNotSwitchOrPersist(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{}}
	local := &fakeAssetStore{id: "local"}
	wrapper := NewSwitchableAssetStore(local)
	svc := NewStorage(mem, local, wrapper)
	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return nil, errors.New("boom")
	})

	err := svc.SaveConfig(context.Background(), StorageConfig{Bucket: "b", SecretKey: "sec"})
	if err == nil {
		t.Fatalf("SaveConfig error = nil, want failure")
	}
	if id := activeID(wrapper); id != "local" {
		t.Errorf("active = %q, want local after failed build", id)
	}
	if mem.values[settingS3Configured] != "" || mem.values[settingS3Bucket] != "" {
		t.Errorf("failed save persisted settings: %v", mem.values)
	}
	if svc.config().Bucket != "" {
		t.Errorf("current bucket = %q, want empty", svc.config().Bucket)
	}
}

func TestStorageLoadUsesPersistedConfig(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{
		settingS3Configured: "true",
		settingS3Endpoint:   "e",
		settingS3Region:     "r",
		settingS3AccessKey:  "a",
		settingS3SecretKey:  "sec",
		settingS3Bucket:     "b",
		settingS3UseSSL:     "false",
		settingS3PublicURL:  "p",
	}}
	local := &fakeAssetStore{id: "local"}
	wrapper := NewSwitchableAssetStore(local)
	svc := NewStorage(mem, local, wrapper)
	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return &fakeAssetStore{id: "s3"}, nil
	})

	if err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if id := activeID(wrapper); id != "s3" {
		t.Errorf("active = %q, want s3", id)
	}
	got, has := svc.Config()
	if !has || got.Region != "r" || got.UseSSL || got.Bucket != "b" {
		t.Errorf("config after load = %+v hasSecret=%v", got, has)
	}
}

func TestStorageLoadWithoutConfigKeepsEnvDefault(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{}}
	local := &fakeAssetStore{id: "local"}
	envS3 := &fakeAssetStore{id: "env-s3"}
	wrapper := NewSwitchableAssetStore(envS3)
	svc := NewStorage(mem, local, wrapper)

	if err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if id := activeID(wrapper); id != "env-s3" {
		t.Errorf("active = %q, want env-s3 (unchanged)", id)
	}
}

func TestStorageTestConnection(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{}}
	local := &fakeAssetStore{id: "local"}
	svc := NewStorage(mem, local, NewSwitchableAssetStore(local))
	ctx := context.Background()

	if err := svc.Test(ctx); err == nil {
		t.Errorf("Test without bucket = nil, want error")
	}

	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return &fakeAssetStore{id: "s3"}, nil
	})
	if err := svc.SaveConfig(ctx, StorageConfig{Bucket: "b", SecretKey: "s"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := svc.Test(ctx); err != nil {
		t.Errorf("Test = %v, want nil", err)
	}

	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return nil, errors.New("unreachable")
	})
	if err := svc.Test(ctx); err == nil {
		t.Errorf("Test with failing factory = nil, want error")
	}
	if id := activeID(svc.target); id != "s3" {
		t.Errorf("Test changed active store to %q", id)
	}
}

func TestStorageTestConfigProbesFormValueAndKeepsState(t *testing.T) {
	mem := &storageMemStore{values: map[string]string{}}
	local := &fakeAssetStore{id: "local"}
	wrapper := NewSwitchableAssetStore(local)
	svc := NewStorage(mem, local, wrapper)
	ctx := context.Background()

	var probed StorageConfig
	svc.SetStoreFactory(func(_ context.Context, cfg StorageConfig) (core.AssetStore, error) {
		probed = cfg
		return &fakeAssetStore{id: "s3"}, nil
	})
	if err := svc.SaveConfig(ctx, StorageConfig{Endpoint: "saved", Bucket: "saved-b", SecretKey: "stored"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if err := svc.TestConfig(ctx, StorageConfig{Endpoint: "form", Bucket: "form-b", SecretKey: ""}); err != nil {
		t.Fatalf("TestConfig: %v", err)
	}
	if probed.Endpoint != "form" || probed.Bucket != "form-b" || probed.SecretKey != "stored" {
		t.Errorf("probed config = %+v, want form values with stored secret", probed)
	}
	if svc.config().Endpoint != "saved" || svc.config().Bucket != "saved-b" {
		t.Errorf("TestConfig mutated saved config: %+v", svc.config())
	}
	if id := activeID(wrapper); id != "s3" {
		t.Errorf("active store = %q, want s3 (unchanged by TestConfig)", id)
	}

	svc.SetStoreFactory(func(_ context.Context, _ StorageConfig) (core.AssetStore, error) {
		return nil, errors.New("dial tcp 127.0.0.1:1: connectex refused")
	})
	err := svc.TestConfig(ctx, StorageConfig{Endpoint: "127.0.0.1:1", Bucket: "form-b"})
	if err == nil {
		t.Fatalf("TestConfig with failing factory = nil, want error")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") || !strings.Contains(err.Error(), "form-b") {
		t.Errorf("TestConfig error = %q, want endpoint and bucket", err.Error())
	}
	if strings.Contains(err.Error(), "stored") {
		t.Errorf("TestConfig error leaked secret: %q", err.Error())
	}
}
