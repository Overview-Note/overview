package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.13.3", "v0.14.0", -1},
		{"v0.14.0", "v0.13.3", 1},
		{"0.13.3", "v0.13.3", 0},
		{"v1.2", "v1.2.0", 0},
		{"v1.2.10", "v1.2.9", 1},
		{"v2.0.0", "v10.0.0", -1},
		{"v1.0.0-beta.1", "v1.0.0", 0},
		{"dev", "v0.14.0", -1},
		{"v0.14.0", "dev", 1},
		{"dev", "dev", 0},
	}
	for _, tc := range cases {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckLatestReportsNewerRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("request missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.14.0","html_url":"https://example.com/releases/v0.14.0"}`))
	}))
	defer srv.Close()

	latest, url, hasUpdate, err := checkLatest(context.Background(), srv.Client(), srv.URL, "v0.13.3")
	if err != nil {
		t.Fatalf("checkLatest: %v", err)
	}
	if latest != "v0.14.0" {
		t.Errorf("latest = %q, want v0.14.0", latest)
	}
	if url != "https://example.com/releases/v0.14.0" {
		t.Errorf("url = %q", url)
	}
	if !hasUpdate {
		t.Error("hasUpdate = false, want true")
	}
}

func TestCheckLatestSameVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.13.3","html_url":"https://example.com"}`))
	}))
	defer srv.Close()

	_, _, hasUpdate, err := checkLatest(context.Background(), srv.Client(), srv.URL, "v0.13.3")
	if err != nil {
		t.Fatalf("checkLatest: %v", err)
	}
	if hasUpdate {
		t.Error("hasUpdate = true, want false")
	}
}

func TestCheckLatestErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, _, _, err := checkLatest(context.Background(), srv.Client(), srv.URL, "v0.13.3"); err == nil {
		t.Fatal("checkLatest succeeded on 403, want error")
	}
}
