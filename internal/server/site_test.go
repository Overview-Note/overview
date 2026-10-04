package server_test

import (
	"context"
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

// newSiteServer builds a multi-mode server with the site settings wired. When
// mail is true an SMTP transport plus capture hook is installed so
// registration requires email verification.
func newSiteServer(t *testing.T, mail bool) (*httptest.Server, *http.Client, *serverMailCapture) {
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
	svc := service.New(st, ix, st, nil, nil)
	auth := service.NewAuth(ix, "multi")

	cap := &serverMailCapture{}
	var mailSvc *service.MailService
	if mail {
		mailSvc = service.NewMail(ix, service.MailConfig{
			Host: "smtp.example.test",
			From: "noreply@example.test",
		})
		mailSvc.SetSendHook(cap.send)
	} else {
		mailSvc = service.NewMail(ix, service.MailConfig{})
	}
	auth.SetMailer(mailSvc)

	siteSvc := service.NewSite(ix)
	siteSvc.Load(context.Background())

	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Auth:           auth,
		Mail:           mailSvc,
		Site:           siteSvc,
		BaseURL:        "https://app.example.test",
	}).Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return ts, &http.Client{Jar: jar}, cap
}

func enableRegistration(t *testing.T, ts *httptest.Server, admin *http.Client) {
	t.Helper()
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/site",
		map[string]any{"registrationEnabled": true}, nil); code != http.StatusOK {
		t.Fatalf("enable registration = %d", code)
	}
}

func TestRegistrationDisabled(t *testing.T) {
	ts, admin, _ := newSiteServer(t, false)
	setupAdmin(t, ts, admin)

	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "bob@example.com", "password": "secret1"}, nil); code != http.StatusForbidden {
		t.Fatalf("register when disabled = %d, want 403", code)
	}
}

func TestRegistrationWithoutMail(t *testing.T) {
	ts, admin, _ := newSiteServer(t, false)
	setupAdmin(t, ts, admin)
	enableRegistration(t, ts, admin)

	var out struct {
		User                 map[string]any `json:"user"`
		VerificationRequired bool           `json:"verificationRequired"`
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "Bob@Example.com", "password": "secret1"}, &out); code != http.StatusOK {
		t.Fatalf("register = %d (%v)", code, out)
	}
	if out.VerificationRequired {
		t.Errorf("verificationRequired = true, want false without mail")
	}
	if out.User["role"] != "member" {
		t.Errorf("role = %v, want member", out.User["role"])
	}
	if out.User["status"] != "active" || out.User["emailVerified"] != true {
		t.Errorf("user = %v, want active + verified", out.User)
	}
	if out.User["email"] != "bob@example.com" {
		t.Errorf("email = %v, want normalised", out.User["email"])
	}

	// The account can sign in immediately.
	guest := &http.Client{}
	if code := authJSON(t, guest, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob@example.com", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("login after registration = %d, want 200", code)
	}
}

func TestRegistrationWithMailRequiresVerification(t *testing.T) {
	ts, admin, cap := newSiteServer(t, true)
	setupAdmin(t, ts, admin)
	enableRegistration(t, ts, admin)

	var out struct {
		User                 map[string]any `json:"user"`
		VerificationRequired bool           `json:"verificationRequired"`
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "carol@example.com", "password": "secret1"}, &out); code != http.StatusOK {
		t.Fatalf("register = %d (%v)", code, out)
	}
	if !out.VerificationRequired {
		t.Fatalf("verificationRequired = false, want true")
	}
	if out.User["emailVerified"] != false {
		t.Errorf("emailVerified = %v, want false", out.User["emailVerified"])
	}

	// Signing in before verifying is refused with a dedicated code.
	var apiErr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "carol@example.com", "password": "secret1"}, &apiErr); code != http.StatusForbidden {
		t.Fatalf("unverified login = %d, want 403", code)
	}
	if apiErr.Error.Code != "email_not_verified" {
		t.Fatalf("error code = %q, want email_not_verified", apiErr.Error.Code)
	}

	// Consuming the verification token unlocks sign-in.
	token := mailToken(cap.last())
	if token == "" {
		t.Fatalf("no verification token: %+v", cap.last())
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/verify-email",
		map[string]string{"token": token}, nil); code != http.StatusOK {
		t.Fatalf("verify email = %d, want 200", code)
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "carol@example.com", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("login after verify = %d, want 200", code)
	}
}

func TestRegistrationDuplicateEmail(t *testing.T) {
	ts, admin, _ := newSiteServer(t, false)
	setupAdmin(t, ts, admin)
	enableRegistration(t, ts, admin)

	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "dup@example.com", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("first register = %d, want 200", code)
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "dup@example.com", "password": "secret1"}, nil); code != http.StatusConflict {
		t.Fatalf("duplicate register = %d, want 409", code)
	}
}

func TestRegistrationValidation(t *testing.T) {
	ts, admin, _ := newSiteServer(t, false)
	setupAdmin(t, ts, admin)
	enableRegistration(t, ts, admin)

	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "not-an-email", "password": "secret1"}, nil); code != http.StatusBadRequest {
		t.Fatalf("invalid email = %d, want 400", code)
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "short@example.com", "password": "123"}, nil); code != http.StatusBadRequest {
		t.Fatalf("short password = %d, want 400", code)
	}
}

func TestResendVerificationNoEnumeration(t *testing.T) {
	ts, admin, cap := newSiteServer(t, true)
	setupAdmin(t, ts, admin)
	enableRegistration(t, ts, admin)

	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/register",
		map[string]string{"email": "dave@example.com", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("register = %d", code)
	}
	before := len(cap.msgs)

	var known, unknown map[string]any
	codeKnown := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/verify/resend",
		map[string]string{"email": "dave@example.com"}, &known)
	codeUnknown := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/verify/resend",
		map[string]string{"email": "nobody@example.com"}, &unknown)
	if codeKnown != http.StatusOK || codeUnknown != http.StatusOK {
		t.Fatalf("resend codes = %d/%d, want 200/200", codeKnown, codeUnknown)
	}
	if len(cap.msgs) <= before {
		t.Fatalf("no verification email resent for a known address")
	}
}

func TestSiteSettingsAdminOnly(t *testing.T) {
	ts, admin, _ := newSiteServer(t, false)
	setupAdmin(t, ts, admin)
	createActiveUser(t, ts, admin, "bob", "secret1")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}
	if code := authJSON(t, member, http.MethodGet, ts.URL+"/api/v1/settings/site", nil, nil); code != http.StatusForbidden {
		t.Fatalf("member get site = %d, want 403", code)
	}
	if code := authJSON(t, member, http.MethodPut, ts.URL+"/api/v1/settings/site",
		map[string]any{"registrationEnabled": true}, nil); code != http.StatusForbidden {
		t.Fatalf("member put site = %d, want 403", code)
	}

	var cfg map[string]any
	if code := authJSON(t, admin, http.MethodGet, ts.URL+"/api/v1/settings/site", nil, &cfg); code != http.StatusOK {
		t.Fatalf("admin get site = %d", code)
	}
	if cfg["registrationEnabled"] != false {
		t.Errorf("default registrationEnabled = %v, want false", cfg["registrationEnabled"])
	}

	var saved map[string]any
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/site", map[string]any{
		"registrationEnabled": true,
		"loginHint":           "Welcome",
		"loginIcp":            "ICP-123",
		"loginLinkUrl":        "https://example.com",
		"loginLinkText":       "Help",
	}, &saved); code != http.StatusOK {
		t.Fatalf("admin save site = %d", code)
	}
	if saved["registrationEnabled"] != true || saved["loginHint"] != "Welcome" ||
		saved["loginIcp"] != "ICP-123" || saved["loginLinkUrl"] != "https://example.com" ||
		saved["loginLinkText"] != "Help" {
		t.Fatalf("saved site = %v", saved)
	}
}

func TestAuthStateIncludesSiteFields(t *testing.T) {
	ts, admin, _ := newSiteServer(t, false)
	setupAdmin(t, ts, admin)

	var state map[string]any
	if code := authJSON(t, http.DefaultClient, http.MethodGet, ts.URL+"/api/v1/auth/state", nil, &state); code != http.StatusOK {
		t.Fatalf("state = %d", code)
	}
	if state["registrationEnabled"] != false || state["loginHint"] != "" || state["loginIcp"] != "" {
		t.Fatalf("default state fields = %v", state)
	}
	if _, ok := state["loginLink"]; ok {
		t.Errorf("loginLink should be absent when no URL is set")
	}

	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/settings/site", map[string]any{
		"registrationEnabled": true,
		"loginHint":           "Sign in here",
		"loginIcp":            "ICP-999",
		"loginLinkUrl":        "https://example.org",
		"loginLinkText":       "Docs",
	}, nil); code != http.StatusOK {
		t.Fatalf("save site = %d", code)
	}

	if code := authJSON(t, http.DefaultClient, http.MethodGet, ts.URL+"/api/v1/auth/state", nil, &state); code != http.StatusOK {
		t.Fatalf("state after save = %d", code)
	}
	if state["registrationEnabled"] != true || state["loginHint"] != "Sign in here" || state["loginIcp"] != "ICP-999" {
		t.Fatalf("state after save = %v", state)
	}
	link, ok := state["loginLink"].(map[string]any)
	if !ok || link["url"] != "https://example.org" || link["text"] != "Docs" {
		t.Fatalf("loginLink = %v", state["loginLink"])
	}
}

// Public registration must not weaken render mode: state stays reachable while
// writes to allowlisted API paths are still rejected with 405.
func TestRenderModeRegisterUnaffected(t *testing.T) {
	ts := newRenderServer(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/state", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /auth/state = %d, want 405", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/v1/auth/state")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /auth/state = %d, want 200", resp.StatusCode)
	}
}
