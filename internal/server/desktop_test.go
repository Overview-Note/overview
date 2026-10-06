package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
)

type fakeHooks struct {
	autostart bool
	dataDir   string
	setErr    error

	vaultErr    error
	vaultResult bool
	vaultCalled string
	restarted   bool
	restartErr  error
	opened      bool
	openErr     error
}

func (h *fakeHooks) AutostartEnabled() bool { return h.autostart }

func (h *fakeHooks) SetAutostart(on bool) error {
	if h.setErr != nil {
		return h.setErr
	}
	h.autostart = on
	return nil
}

func (h *fakeHooks) DataDir() string { return h.dataDir }

func (h *fakeHooks) SetVault(dataDir string) (string, bool, error) {
	h.vaultCalled = dataDir
	if h.vaultErr != nil {
		return "", false, h.vaultErr
	}
	if !h.vaultResult {
		return "", false, nil
	}
	// The running server keeps serving the old directory until it restarts,
	// so DataDir() intentionally stays put; only pendingDataDir moves.
	return dataDir, true, nil
}

func (h *fakeHooks) Restart() error {
	if h.restartErr != nil {
		return h.restartErr
	}
	h.restarted = true
	return nil
}

func (h *fakeHooks) OpenDataDir() error {
	if h.openErr != nil {
		return h.openErr
	}
	h.opened = true
	return nil
}

func newDesktopServer(t *testing.T, hooks server.DesktopHooks, check server.UpdateChecker) *httptest.Server {
	t.Helper()
	s := server.New(nil, server.Options{
		LocalOnly:    true,
		Auth:         service.NewAuth(nil, "none"),
		Version:      "0.13.3",
		DesktopHooks: hooks,
		UpdateCheck:  check,
	})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestDesktopSettingsRoundTrip(t *testing.T) {
	hooks := &fakeHooks{dataDir: "/vault"}
	ts := newDesktopServer(t, hooks, nil)

	resp, err := http.Get(ts.URL + "/api/v1/desktop/settings")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got["autostart"] != false || got["dataDir"] != "/vault" || got["version"] != "0.13.3" {
		t.Fatalf("settings = %v", got)
	}

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/desktop/settings",
		strings.NewReader(`{"autostart":true}`))
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", putResp.StatusCode)
	}
	var updated map[string]any
	if err := json.NewDecoder(putResp.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated["autostart"] != true {
		t.Errorf("autostart = %v, want true", updated["autostart"])
	}
	if !hooks.autostart {
		t.Error("hooks.autostart = false, want true")
	}
}

func TestDesktopVaultPut(t *testing.T) {
	hooks := &fakeHooks{dataDir: "/old", vaultResult: true}
	ts := newDesktopServer(t, hooks, nil)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/desktop/vault",
		strings.NewReader(`{"dataDir":"/new"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["changed"] != true || got["dataDir"] != "/old" || got["pendingDataDir"] != "/new" {
		t.Errorf("vault response = %v", got)
	}
	if hooks.vaultCalled != "/new" {
		t.Errorf("vaultCalled = %q, want /new", hooks.vaultCalled)
	}
}

func TestDesktopVaultPutCancelled(t *testing.T) {
	hooks := &fakeHooks{dataDir: "/old"}
	ts := newDesktopServer(t, hooks, nil)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/desktop/vault",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["changed"] != false || got["dataDir"] != "/old" {
		t.Errorf("vault response = %v", got)
	}
}

func TestDesktopVaultPutInvalid(t *testing.T) {
	hooks := &fakeHooks{vaultErr: errors.New("not a vault")}
	ts := newDesktopServer(t, hooks, nil)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/desktop/vault",
		strings.NewReader(`{"dataDir":"/bad"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDesktopRestartAndOpenFolder(t *testing.T) {
	hooks := &fakeHooks{}
	ts := newDesktopServer(t, hooks, nil)

	for _, tc := range []struct {
		path   string
		verify func() bool
	}{
		{"/api/v1/desktop/restart", func() bool { return hooks.restarted }},
		{"/api/v1/desktop/open-folder", func() bool { return hooks.opened }},
	} {
		resp, err := http.Post(ts.URL+tc.path, "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST %s = %d, want 200", tc.path, resp.StatusCode)
		}
		if !tc.verify() {
			t.Errorf("POST %s did not invoke the hook", tc.path)
		}
	}
}

func TestDesktopUpdateUsesChecker(t *testing.T) {
	check := func(context.Context, string) (string, string, bool, error) {
		return "v0.14.0", "https://example.com/releases/v0.14.0", true, nil
	}
	ts := newDesktopServer(t, &fakeHooks{}, check)

	resp, err := http.Get(ts.URL + "/api/v1/desktop/update")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["current"] != "0.13.3" || got["latest"] != "v0.14.0" || got["hasUpdate"] != true {
		t.Errorf("update = %v", got)
	}
	if got["url"] != "https://example.com/releases/v0.14.0" {
		t.Errorf("url = %v", got["url"])
	}
}

func TestDesktopEndpointsHiddenWhenHeadless(t *testing.T) {
	s := server.New(nil, server.Options{Auth: service.NewAuth(nil, "none")})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	for _, path := range []string{
		"/api/v1/desktop/settings",
		"/api/v1/desktop/update",
		"/api/v1/desktop/vault",
		"/api/v1/desktop/restart",
		"/api/v1/desktop/open-folder",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("headless GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
}
