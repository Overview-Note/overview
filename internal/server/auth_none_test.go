package server_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

// newNoAuthServer builds a single-user server (auth disabled) with every
// admin-gated service wired so the settings, users and tokens endpoints are
// fully reachable without a session.
func newNoAuthServer(t *testing.T) *httptest.Server {
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

	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Auth:           service.NewAuth(ix, "none"),
		Tokens:         service.NewTokenService(ix),
		AI:             service.NewAI(ix, service.AIConfig{Model: "gpt-4o-mini"}),
		Mail:           service.NewMail(ix, service.MailConfig{}),
		Site:           service.NewSite(ix),
		Storage:        service.NewStorage(ix, st, wrapper),
	}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

// TestNoAuthOwnerReachesAdminEndpoints verifies that when authentication is
// disabled the anonymous caller acts as the owner and can read and write every
// admin-gated endpoint.
func TestNoAuthOwnerReachesAdminEndpoints(t *testing.T) {
	ts := newNoAuthServer(t)

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/v1/settings/ai", nil},
		{http.MethodPut, "/api/v1/settings/ai", map[string]any{"model": "gpt-4o-mini"}},
		{http.MethodGet, "/api/v1/settings/mail", nil},
		{http.MethodPut, "/api/v1/settings/mail", map[string]any{"host": "smtp.example.test", "port": 587}},
		{http.MethodGet, "/api/v1/settings/site", nil},
		{http.MethodPut, "/api/v1/settings/site", map[string]any{"loginHint": "welcome"}},
		{http.MethodGet, "/api/v1/settings/storage", nil},
		{http.MethodPut, "/api/v1/settings/storage", map[string]any{}},
		{http.MethodGet, "/api/v1/auth/users", nil},
		{http.MethodGet, "/api/v1/auth/tokens", nil},
	}
	for _, c := range cases {
		resp, body := doJSON(t, c.method, ts.URL+c.path, c.body)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s %s = %d, want 200 (%v)", c.method, c.path, resp.StatusCode, body)
		}
	}

	// The implicit identity is the owner administrator.
	resp, me := doJSON(t, http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
	if resp.StatusCode != http.StatusOK || me["username"] != "owner" || me["role"] != "admin" {
		t.Errorf("auth/me = %d %v, want owner/admin", resp.StatusCode, me)
	}
}

// TestMultiAuthStillGatesAdminEndpoints guards against regressions: with
// authentication in multi mode an anonymous caller is rejected and a plain
// member is forbidden, while an administrator is allowed.
func TestMultiAuthStillGatesAdminEndpoints(t *testing.T) {
	ts, admin := newAuthServer(t)

	adminPaths := []string{
		"/api/v1/settings/ai",
		"/api/v1/settings/mail",
		"/api/v1/settings/site",
		"/api/v1/settings/storage",
		"/api/v1/auth/users",
		"/api/v1/auth/tokens",
	}

	for _, path := range adminPaths {
		if code := doJSONStatus(t, http.MethodGet, ts.URL+path, nil); code != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s = %d, want 401", path, code)
		}
	}

	setupAdmin(t, ts, admin)
	createActiveUser(t, ts, admin, "bob", "secret1")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}
	for _, path := range adminPaths {
		if code := authJSON(t, member, http.MethodGet, ts.URL+path, nil, nil); code != http.StatusForbidden {
			t.Errorf("member GET %s = %d, want 403", path, code)
		}
	}
	if code := authJSON(t, admin, http.MethodGet, ts.URL+"/api/v1/settings/mail", nil, nil); code != http.StatusOK {
		t.Errorf("admin GET settings/mail = %d, want 200", code)
	}
}

func doJSONStatus(t *testing.T, method, url string, body any) int {
	t.Helper()
	resp, _ := doJSON(t, method, url, body)
	return resp.StatusCode
}
