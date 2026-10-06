package server_test

import (
	"net/http"
	"net/http/cookiejar"
	"testing"
)

// TestAuthFailuresUseSpecificErrorCodes verifies that a 401 and a 403 carry
// dedicated error codes rather than the generic "internal" fallback.
func TestAuthFailuresUseSpecificErrorCodes(t *testing.T) {
	ts, admin := newAuthServer(t)

	var anon map[string]any
	if code := authJSON(t, &http.Client{}, http.MethodGet, ts.URL+"/api/v1/tree", nil, &anon); code != http.StatusUnauthorized {
		t.Fatalf("anonymous tree = %d, want 401 (%v)", code, anon)
	}
	if got := apiErrorCode(anon); got != "unauthorized" {
		t.Errorf("anonymous error code = %q, want unauthorized (%v)", got, anon)
	}

	setupAdmin(t, ts, admin)
	createActiveUser(t, ts, admin, "bob", "secret1")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}

	var forbidden map[string]any
	if code := authJSON(t, member, http.MethodGet, ts.URL+"/api/v1/settings/ai", nil, &forbidden); code != http.StatusForbidden {
		t.Fatalf("member settings/ai = %d, want 403 (%v)", code, forbidden)
	}
	if got := apiErrorCode(forbidden); got != "forbidden" {
		t.Errorf("member error code = %q, want forbidden (%v)", got, forbidden)
	}
}

func apiErrorCode(body map[string]any) string {
	errObj, _ := body["error"].(map[string]any)
	code, _ := errObj["code"].(string)
	return code
}
