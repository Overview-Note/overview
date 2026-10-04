package server_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/server"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
)

type serverMailCapture struct {
	mu   sync.Mutex
	msgs []service.Message
}

func (c *serverMailCapture) send(_ context.Context, m service.Message) error {
	c.mu.Lock()
	c.msgs = append(c.msgs, m)
	c.mu.Unlock()
	return nil
}

func (c *serverMailCapture) last() service.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.msgs) == 0 {
		return service.Message{}
	}
	return c.msgs[len(c.msgs)-1]
}

var mailTokenPattern = regexp.MustCompile(`token=([0-9a-f]+)`)

func mailToken(m service.Message) string {
	if mm := mailTokenPattern.FindStringSubmatch(m.Text); len(mm) == 2 {
		return mm[1]
	}
	if mm := mailTokenPattern.FindStringSubmatch(m.HTML); len(mm) == 2 {
		return mm[1]
	}
	return ""
}

func newEmailAuthServer(t *testing.T) (*httptest.Server, *http.Client, *serverMailCapture) {
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
	mailSvc := service.NewMail(ix, service.MailConfig{
		Host: "smtp.example.test",
		From: "noreply@example.test",
	})
	mailSvc.SetSendHook(cap.send)
	auth.SetMailer(mailSvc)
	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Auth:           auth,
		Mail:           mailSvc,
		BaseURL:        "https://app.example.test",
	}).Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return ts, &http.Client{Jar: jar}, cap
}

func newDisabledMailServer(t *testing.T) (*httptest.Server, *http.Client) {
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
	mailSvc := service.NewMail(ix, service.MailConfig{})
	auth.SetMailer(mailSvc)
	ts := httptest.NewServer(server.New(svc, server.Options{
		MaxUploadBytes: 1 << 20,
		Auth:           auth,
		Mail:           mailSvc,
		BaseURL:        "https://app.example.test",
	}).Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return ts, &http.Client{Jar: jar}
}

func setupAdmin(t *testing.T, ts *httptest.Server, client *http.Client) {
	t.Helper()
	var out map[string]any
	if code := authJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/setup",
		map[string]string{"username": "admin", "password": "secret1"}, &out); code != http.StatusOK {
		t.Fatalf("setup = %d (%v)", code, out)
	}
}

func inviteAndAccept(t *testing.T, ts *httptest.Server, admin *http.Client, cap *serverMailCapture, email string) string {
	t.Helper()
	var invited struct {
		User map[string]any `json:"user"`
	}
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/auth/users/invite",
		map[string]string{"email": email, "role": "member"}, &invited); code != http.StatusOK {
		t.Fatalf("invite = %d (%v)", code, invited)
	}
	token := mailToken(cap.last())
	if token == "" {
		t.Fatalf("no invitation token: %+v", cap.last())
	}
	var accepted struct {
		User map[string]any `json:"user"`
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/accept-invite",
		map[string]string{"token": token, "password": "newsecret"}, &accepted); code != http.StatusOK {
		t.Fatalf("accept = %d (%v)", code, accepted)
	}
	return token
}

func TestEmailInviteAcceptLoginOverHTTP(t *testing.T) {
	ts, client, cap := newEmailAuthServer(t)
	setupAdmin(t, ts, client)

	var invited struct {
		User map[string]any `json:"user"`
	}
	if code := authJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/users/invite",
		map[string]string{"email": "bob@example.com", "role": "member"}, &invited); code != http.StatusOK {
		t.Fatalf("invite = %d (%v)", code, invited)
	}
	if invited.User["status"] != "invited" {
		t.Fatalf("status = %v", invited.User["status"])
	}
	token := mailToken(cap.last())
	if token == "" {
		t.Fatalf("no token in invitation: %+v", cap.last())
	}

	var accepted struct {
		User map[string]any `json:"user"`
	}
	guest := &http.Client{}
	if code := authJSON(t, guest, http.MethodPost, ts.URL+"/api/v1/auth/accept-invite",
		map[string]string{"token": token, "password": "newsecret"}, &accepted); code != http.StatusOK {
		t.Fatalf("accept = %d (%v)", code, accepted)
	}
	if accepted.User["status"] != "active" || accepted.User["emailVerified"] != true {
		t.Fatalf("accepted user = %v", accepted.User)
	}

	var login map[string]any
	if code := authJSON(t, guest, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob@example.com", "password": "newsecret"}, &login); code != http.StatusOK {
		t.Fatalf("login with email = %d (%v)", code, login)
	}

	// A spent invitation link is rejected.
	if code := authJSON(t, guest, http.MethodPost, ts.URL+"/api/v1/auth/accept-invite",
		map[string]string{"token": token, "password": "another1"}, nil); code != http.StatusBadRequest {
		t.Fatalf("token reuse = %d, want 400", code)
	}

	// Invalid tokens share the same generic 400.
	if code := authJSON(t, guest, http.MethodPost, ts.URL+"/api/v1/auth/accept-invite",
		map[string]string{"token": "deadbeef", "password": "another1"}, nil); code != http.StatusBadRequest {
		t.Fatalf("invalid token = %d, want 400", code)
	}
}

func TestPasswordRequestConstantResponse(t *testing.T) {
	ts, client, cap := newEmailAuthServer(t)
	setupAdmin(t, ts, client)
	inviteAndAccept(t, ts, client, cap, "bob@example.com")

	var known, unknown map[string]any
	codeKnown := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/password/request",
		map[string]string{"email": "bob@example.com"}, &known)
	codeUnknown := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/password/request",
		map[string]string{"email": "nobody@example.com"}, &unknown)
	if codeKnown != http.StatusOK || codeUnknown != http.StatusOK {
		t.Fatalf("codes = %d/%d, want 200/200", codeKnown, codeUnknown)
	}
	if !reflect.DeepEqual(known, unknown) {
		t.Fatalf("responses differ: %v vs %v", known, unknown)
	}
}

func TestMemberCannotInvite(t *testing.T) {
	ts, admin, cap := newEmailAuthServer(t)
	setupAdmin(t, ts, admin)
	inviteAndAccept(t, ts, admin, cap, "bob@example.com")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob@example.com", "password": "newsecret"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/users/invite",
		map[string]string{"email": "carol@example.com", "role": "member"}, nil); code != http.StatusForbidden {
		t.Fatalf("member invite = %d, want 403", code)
	}
}

func TestInviteWithoutMailConfigured(t *testing.T) {
	ts, client := newDisabledMailServer(t)
	setupAdmin(t, ts, client)
	var state map[string]any
	authJSON(t, client, http.MethodGet, ts.URL+"/api/v1/auth/state", nil, &state)
	if state["mailEnabled"] != false {
		t.Errorf("mailEnabled = %v, want false", state["mailEnabled"])
	}
	if code := authJSON(t, client, http.MethodPost, ts.URL+"/api/v1/auth/users/invite",
		map[string]string{"email": "bob@example.com", "role": "member"}, nil); code != http.StatusServiceUnavailable {
		t.Fatalf("invite with mail disabled = %d, want 503", code)
	}
}

func TestMailEnabledState(t *testing.T) {
	ts, client, _ := newEmailAuthServer(t)
	var state map[string]any
	if code := authJSON(t, client, http.MethodGet, ts.URL+"/api/v1/auth/state", nil, &state); code != http.StatusOK {
		t.Fatalf("state = %d", code)
	}
	if state["mailEnabled"] != true {
		t.Fatalf("mailEnabled = %v, want true", state["mailEnabled"])
	}
}

func TestResetPasswordInvalidatesSessionOverHTTP(t *testing.T) {
	ts, admin, cap := newEmailAuthServer(t)
	setupAdmin(t, ts, admin)
	inviteAndAccept(t, ts, admin, cap, "bob@example.com")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob@example.com", "password": "newsecret"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}
	if code := authJSON(t, member, http.MethodGet, ts.URL+"/api/v1/auth/me", nil, nil); code != http.StatusOK {
		t.Fatalf("me before reset = %d", code)
	}

	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/password/request",
		map[string]string{"email": "bob@example.com"}, nil); code != http.StatusOK {
		t.Fatalf("request reset = %d", code)
	}
	resetToken := mailToken(cap.last())
	if resetToken == "" {
		t.Fatalf("no reset token: %+v", cap.last())
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/password/reset",
		map[string]string{"token": resetToken, "password": "brandnew1"}, nil); code != http.StatusOK {
		t.Fatalf("reset = %d", code)
	}
	if code := authJSON(t, member, http.MethodGet, ts.URL+"/api/v1/auth/me", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("me after reset = %d, want 401", code)
	}
}

func createActiveUser(t *testing.T, ts *httptest.Server, admin *http.Client, username, password string) string {
	t.Helper()
	var created map[string]any
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/auth/users",
		map[string]string{"username": username, "password": password, "role": "member"}, &created); code != http.StatusOK {
		t.Fatalf("create user = %d (%v)", code, created)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("created user has no id: %v", created)
	}
	return id
}

func listUser(t *testing.T, ts *httptest.Server, client *http.Client, id string) map[string]any {
	t.Helper()
	var list struct {
		Users []map[string]any `json:"users"`
	}
	if code := authJSON(t, client, http.MethodGet, ts.URL+"/api/v1/auth/users", nil, &list); code != http.StatusOK {
		t.Fatalf("list users = %d", code)
	}
	for _, u := range list.Users {
		if u["id"] == id {
			return u
		}
	}
	t.Fatalf("user %s not found", id)
	return nil
}

func TestAdminSetEmailSendsVerification(t *testing.T) {
	ts, admin, cap := newEmailAuthServer(t)
	setupAdmin(t, ts, admin)
	id := createActiveUser(t, ts, admin, "bob", "secret1")

	var out struct {
		User             map[string]any `json:"user"`
		VerificationSent bool           `json:"verificationSent"`
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/"+id+"/email",
		map[string]string{"email": "Bob@Example.com"}, &out); code != http.StatusOK {
		t.Fatalf("set email = %d (%v)", code, out)
	}
	if !out.VerificationSent {
		t.Fatalf("verificationSent = false, want true")
	}
	if out.User["email"] != "bob@example.com" {
		t.Errorf("email = %v, want normalised lower-case", out.User["email"])
	}
	if out.User["emailVerified"] != false {
		t.Errorf("emailVerified = %v, want false", out.User["emailVerified"])
	}

	msg := cap.last()
	if msg.To != "bob@example.com" {
		t.Errorf("mail to = %q, want bob@example.com", msg.To)
	}
	if !strings.Contains(msg.Text, "https://app.example.test/verify-email#token=") {
		t.Errorf("verification link missing in %q", msg.Text)
	}
	token := mailToken(msg)
	if token == "" {
		t.Fatalf("no verification token: %+v", msg)
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/verify-email",
		map[string]string{"token": token}, nil); code != http.StatusOK {
		t.Fatalf("verify email = %d, want 200", code)
	}
	if got := listUser(t, ts, admin, id)["emailVerified"]; got != true {
		t.Fatalf("emailVerified after verify = %v, want true", got)
	}
}

func TestSetEmailValidationAndConflicts(t *testing.T) {
	ts, admin, _ := newEmailAuthServer(t)
	setupAdmin(t, ts, admin)
	id := createActiveUser(t, ts, admin, "bob", "secret1")
	other := createActiveUser(t, ts, admin, "carol", "secret1")

	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/"+id+"/email",
		map[string]string{"email": "not-an-email"}, nil); code != http.StatusBadRequest {
		t.Fatalf("invalid email = %d, want 400", code)
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/"+id+"/email",
		map[string]string{"email": "shared@example.com"}, nil); code != http.StatusOK {
		t.Fatalf("set first email = %d, want 200", code)
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/"+other+"/email",
		map[string]string{"email": "shared@example.com"}, nil); code != http.StatusConflict {
		t.Fatalf("duplicate email = %d, want 409", code)
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/missing/email",
		map[string]string{"email": "x@example.com"}, nil); code != http.StatusNotFound {
		t.Fatalf("missing user = %d, want 404", code)
	}
}

func TestSendVerificationEndpoint(t *testing.T) {
	ts, admin, cap := newEmailAuthServer(t)
	setupAdmin(t, ts, admin)
	id := createActiveUser(t, ts, admin, "bob", "secret1")

	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/auth/users/"+id+"/send-verification", nil, nil); code != http.StatusConflict {
		t.Fatalf("send without email = %d, want 409", code)
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/"+id+"/email",
		map[string]string{"email": "bob@example.com"}, nil); code != http.StatusOK {
		t.Fatalf("set email = %d, want 200", code)
	}
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/auth/users/"+id+"/send-verification", nil, nil); code != http.StatusOK {
		t.Fatalf("send verification = %d, want 200", code)
	}
	token := mailToken(cap.last())
	if token == "" {
		t.Fatalf("no verification token: %+v", cap.last())
	}
	if code := authJSON(t, http.DefaultClient, http.MethodPost, ts.URL+"/api/v1/auth/verify-email",
		map[string]string{"token": token}, nil); code != http.StatusOK {
		t.Fatalf("verify = %d, want 200", code)
	}
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/auth/users/"+id+"/send-verification", nil, nil); code != http.StatusConflict {
		t.Fatalf("send for verified user = %d, want 409", code)
	}
}

func TestMemberCannotManageEmail(t *testing.T) {
	ts, admin, _ := newEmailAuthServer(t)
	setupAdmin(t, ts, admin)
	id := createActiveUser(t, ts, admin, "bob", "secret1")

	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "bob", "password": "secret1"}, nil); code != http.StatusOK {
		t.Fatalf("member login = %d", code)
	}
	if code := authJSON(t, member, http.MethodPut, ts.URL+"/api/v1/auth/users/"+id+"/email",
		map[string]string{"email": "bob@example.com"}, nil); code != http.StatusForbidden {
		t.Fatalf("member set email = %d, want 403", code)
	}
	if code := authJSON(t, member, http.MethodPost, ts.URL+"/api/v1/auth/users/"+id+"/send-verification", nil, nil); code != http.StatusForbidden {
		t.Fatalf("member send verification = %d, want 403", code)
	}
}

func TestEmailVerificationWithoutMailConfigured(t *testing.T) {
	ts, admin := newDisabledMailServer(t)
	setupAdmin(t, ts, admin)
	id := createActiveUser(t, ts, admin, "bob", "secret1")

	var out struct {
		User             map[string]any `json:"user"`
		VerificationSent bool           `json:"verificationSent"`
	}
	if code := authJSON(t, admin, http.MethodPut, ts.URL+"/api/v1/auth/users/"+id+"/email",
		map[string]string{"email": "bob@example.com"}, &out); code != http.StatusOK {
		t.Fatalf("set email = %d (%v), want 200", code, out)
	}
	if out.VerificationSent {
		t.Errorf("verificationSent = true, want false")
	}
	if out.User["email"] != "bob@example.com" {
		t.Errorf("email not saved: %v", out.User["email"])
	}
	if code := authJSON(t, admin, http.MethodPost, ts.URL+"/api/v1/auth/users/"+id+"/send-verification", nil, nil); code != http.StatusServiceUnavailable {
		t.Fatalf("send verification with mail disabled = %d, want 503", code)
	}
}
