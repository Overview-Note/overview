package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
)

func newLocalServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := server.New(nil, server.Options{
		LocalOnly: true,
		Auth:      service.NewAuth(nil, "none"),
	})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestLocalGuardHost(t *testing.T) {
	ts := newLocalServer(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/health", nil)
	req.Host = "evil.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("forged host status = %d, want 403", resp.StatusCode)
	}

	resp2, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("loopback host status = %d, want 200", resp2.StatusCode)
	}
}

func TestLocalGuardOrigin(t *testing.T) {
	ts := newLocalServer(t)

	post := func(origin string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/anything", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("http://evil.example"); code != http.StatusForbidden {
		t.Errorf("cross-origin POST = %d, want 403", code)
	}
	if code := post(ts.URL); code == http.StatusForbidden {
		t.Errorf("same-origin POST = %d, want not 403", code)
	}
	if code := post(""); code == http.StatusForbidden {
		t.Errorf("origin-less POST = %d, want not 403", code)
	}

	// GET requests are not subject to the origin check.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("cross-origin GET = %d, want 200", resp.StatusCode)
	}
}
