package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

type storageFakeStore struct{}

func (s *storageFakeStore) Save(_ context.Context, _ string, _ io.Reader) (core.Asset, error) {
	return core.Asset{}, nil
}
func (s *storageFakeStore) Open(_ context.Context, _ string) (io.ReadSeekCloser, error) {
	return nil, core.ErrNotFound
}
func (s *storageFakeStore) ListAssets(_ context.Context) ([]core.Asset, error) { return nil, nil }
func (s *storageFakeStore) DeleteAsset(_ context.Context, _ string) error      { return nil }

func newStorageServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	root := t.TempDir()
	notes := filepath.Join(root, "notes")
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	st := store.New(notes, assets)
	wrapper := service.NewSwitchableAssetStore(st)
	svc := service.New(st, ix, wrapper, nil, nil)
	auth := service.NewAuth(ix, "multi")

	storageSvc := service.NewStorage(ix, st, wrapper)
	storageSvc.SetStoreFactory(func(_ context.Context, cfg service.StorageConfig) (core.AssetStore, error) {
		if cfg.Bucket == "bad" {
			return nil, errors.New("connection refused")
		}
		return &storageFakeStore{}, nil
	})

	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Auth:           auth,
		Storage:        storageSvc,
	}).Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return ts, &http.Client{Jar: jar}
}

func TestStorageSettingsAdminOnly(t *testing.T) {
	ts, admin := newStorageServer(t)
	setupAdmin(t, ts, admin)
	createActiveUser(t, ts, admin, "bob", "secret1")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}
	if code := authJSON(t, member, http.MethodGet, ts.URL+"/api/v1/settings/storage", nil, nil); code != http.StatusForbidden {
		t.Errorf("member get storage = %d, want 403", code)
	}
	if code := authJSON(t, member, http.MethodPut, ts.URL+"/api/v1/settings/storage",
		map[string]any{"bucket": "b"}, nil); code != http.StatusForbidden {
		t.Errorf("member put storage = %d, want 403", code)
	}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/settings/storage/test", nil, nil); code != http.StatusForbidden {
		t.Errorf("member test storage = %d, want 403", code)
	}

	var initial map[string]any
	if code := authJSON(t, admin, http.MethodGet, ts.URL+"/api/v1/settings/storage", nil, &initial); code != http.StatusOK {
		t.Fatalf("admin get storage = %d", code)
	}
	if initial["enabled"] != false || initial["hasSecret"] != false {
		t.Errorf("initial storage = %v", initial)
	}
	if _, leaked := initial["secretKey"]; leaked {
		t.Errorf("secretKey must not be returned: %v", initial)
	}

	var saved map[string]any
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/storage", map[string]any{
		"endpoint":  "minio.local:9000",
		"region":    "us-east-1",
		"accessKey": "key",
		"secretKey": "super-secret",
		"bucket":    "assets",
		"useSSL":    true,
		"publicURL": "https://cdn.example.test",
	}, &saved); code != http.StatusOK {
		t.Fatalf("admin save storage = %d (%v)", code, saved)
	}
	if saved["enabled"] != true || saved["hasSecret"] != true || saved["bucket"] != "assets" {
		t.Fatalf("saved storage = %v", saved)
	}
	if _, leaked := saved["secretKey"]; leaked {
		t.Errorf("secretKey must not be returned: %v", saved)
	}

	var reread map[string]any
	authJSON(t, admin, http.MethodGet, ts.URL+"/api/v1/settings/storage", nil, &reread)
	if reread["hasSecret"] != true || reread["enabled"] != true {
		t.Errorf("reread storage = %v", reread)
	}
	if _, leaked := reread["secretKey"]; leaked {
		t.Errorf("secretKey must not be returned: %v", reread)
	}

	var test map[string]any
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/settings/storage/test", nil, &test); code != http.StatusOK {
		t.Fatalf("test storage = %d", code)
	}
	if test["ok"] != true {
		t.Errorf("test result = %v, want ok", test)
	}
}

func TestStorageTestUsesFormValueWithoutSaving(t *testing.T) {
	ts, admin := newStorageServer(t)
	setupAdmin(t, ts, admin)

	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/storage",
		map[string]any{"endpoint": "saved.local:9000", "bucket": "assets", "secretKey": "s"}, nil); code != http.StatusOK {
		t.Fatalf("initial save = %d", code)
	}

	var result map[string]any
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/settings/storage/test",
		map[string]any{"endpoint": "127.0.0.1:1", "bucket": "bad", "secretKey": ""}, &result); code != http.StatusOK {
		t.Fatalf("test storage = %d", code)
	}
	if result["ok"] != false {
		t.Errorf("test result = %v, want ok:false", result)
	}
	msg, _ := result["message"].(string)
	if !strings.Contains(msg, "127.0.0.1:1") || !strings.Contains(msg, "bad") {
		t.Errorf("test message = %q, want endpoint and bucket", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("test message = %q, want underlying cause", msg)
	}

	var cfg map[string]any
	authJSON(t, admin, http.MethodGet, ts.URL+"/api/v1/settings/storage", nil, &cfg)
	if cfg["endpoint"] != "saved.local:9000" || cfg["bucket"] != "assets" {
		t.Errorf("test mutated saved config: %v", cfg)
	}
}

func TestStorageSettingsInvalidBuildKeepsBackend(t *testing.T) {
	ts, admin := newStorageServer(t)
	setupAdmin(t, ts, admin)

	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/storage",
		map[string]any{"bucket": "assets", "secretKey": "s"}, nil); code != http.StatusOK {
		t.Fatalf("initial save = %d", code)
	}

	var failure struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/storage",
		map[string]any{"endpoint": "127.0.0.1:1", "bucket": "bad", "secretKey": "s"}, &failure); code == http.StatusOK {
		t.Fatalf("invalid build save = %d, want error", code)
	}
	if failure.Error.Message == "" || failure.Error.Message == "internal server error" {
		t.Errorf("save failure message = %q, want a diagnosable message", failure.Error.Message)
	}
	if !strings.Contains(failure.Error.Message, "connection refused") {
		t.Errorf("save failure message = %q, want underlying cause", failure.Error.Message)
	}
	if !strings.Contains(failure.Error.Message, "127.0.0.1:1") || !strings.Contains(failure.Error.Message, "bad") {
		t.Errorf("save failure message = %q, want endpoint and bucket", failure.Error.Message)
	}

	var cfg map[string]any
	authJSON(t, admin, http.MethodGet, ts.URL+"/api/v1/settings/storage", nil, &cfg)
	if cfg["bucket"] != "assets" || cfg["enabled"] != true {
		t.Errorf("failed save mutated config: %v", cfg)
	}

	// Clearing the bucket returns to the local backend.
	var cleared map[string]any
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/storage",
		map[string]any{"bucket": "", "endpoint": "minio.local:9000"}, &cleared); code != http.StatusOK {
		t.Fatalf("clear save = %d", code)
	}
	if cleared["enabled"] != false {
		t.Errorf("enabled after clear = %v, want false", cleared["enabled"])
	}
}
