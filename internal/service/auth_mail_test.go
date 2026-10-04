package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/service"
)

type mailCapture struct {
	mu   sync.Mutex
	msgs []service.Message
}

func (c *mailCapture) send(_ context.Context, m service.Message) error {
	c.mu.Lock()
	c.msgs = append(c.msgs, m)
	c.mu.Unlock()
	return nil
}

func (c *mailCapture) last() service.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.msgs) == 0 {
		return service.Message{}
	}
	return c.msgs[len(c.msgs)-1]
}

func (c *mailCapture) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

func (c *mailCapture) reset() {
	c.mu.Lock()
	c.msgs = nil
	c.mu.Unlock()
}

var tokenPattern = regexp.MustCompile(`token=([0-9a-f]+)`)

func tokenFrom(m service.Message) string {
	if m.Text != "" {
		if mm := tokenPattern.FindStringSubmatch(m.Text); len(mm) == 2 {
			return mm[1]
		}
	}
	if mm := tokenPattern.FindStringSubmatch(m.HTML); len(mm) == 2 {
		return mm[1]
	}
	return ""
}

func newMailAuth(t *testing.T) (*service.AuthService, *index.Index, *mailCapture) {
	t.Helper()
	ix, err := index.Open(filepath.Join(t.TempDir(), "mail.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	auth := service.NewAuth(ix, "multi")
	cap := &mailCapture{}
	mail := service.NewMail(ix, service.MailConfig{})
	mail.SetSendHook(cap.send)
	auth.SetMailer(mail)
	return auth, ix, cap
}

func TestInviteAcceptLoginFlow(t *testing.T) {
	ctx := context.Background()
	auth, _, cap := newMailAuth(t)

	if _, _, err := auth.Setup(ctx, "admin", "secret1"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	user, err := auth.Invite(ctx, "Bob@Example.com", "member", "https://example.test")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if user.Status != core.StatusInvited || user.Email != "bob@example.com" {
		t.Fatalf("unexpected invited user: %+v", user)
	}
	if user.IsAdmin() || user.Role != core.RoleMember {
		t.Fatalf("role = %q", user.Role)
	}

	token := tokenFrom(cap.last())
	if token == "" {
		t.Fatalf("no token in mail: %+v", cap.last())
	}

	if _, _, err := auth.Login(ctx, "bob@example.com", "newsecret"); err == nil {
		t.Error("invited user must not be able to log in")
	}

	activated, err := auth.AcceptInvite(ctx, token, "newsecret")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if activated.Status != core.StatusActive || !activated.EmailVerified {
		t.Fatalf("unexpected activated user: %+v", activated)
	}

	if _, err := auth.AcceptInvite(ctx, token, "another1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("token reuse = %v, want ErrNotFound", err)
	}

	if _, _, err := auth.Login(ctx, "bob@example.com", "newsecret"); err != nil {
		t.Errorf("login after activation: %v", err)
	}
}

func TestResendInviteInvalidatesOldLink(t *testing.T) {
	ctx := context.Background()
	auth, _, cap := newMailAuth(t)
	if _, _, err := auth.Setup(ctx, "admin", "secret1"); err != nil {
		t.Fatal(err)
	}
	user, err := auth.Invite(ctx, "bob@example.com", "member", "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	first := tokenFrom(cap.last())

	if err := auth.ResendInvite(ctx, user.ID, "https://example.test"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	second := tokenFrom(cap.last())
	if second == "" || second == first {
		t.Fatalf("expected a fresh invitation token, got %q -> %q", first, second)
	}
	if _, err := auth.AcceptInvite(ctx, first, "newsecret"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("old invitation token = %v, want ErrNotFound", err)
	}
	activated, err := auth.AcceptInvite(ctx, second, "newsecret")
	if err != nil || activated.Status != core.StatusActive {
		t.Fatalf("accept fresh token: %+v (%v)", activated, err)
	}
	if err := auth.ResendInvite(ctx, user.ID, "https://example.test"); !errors.Is(err, core.ErrConflict) {
		t.Errorf("resend for active user = %v, want conflict", err)
	}
}

func TestInviteDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	auth, _, _ := newMailAuth(t)
	if _, _, err := auth.Setup(ctx, "admin", "secret1"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Invite(ctx, "dup@example.com", "member", "https://example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Invite(ctx, "DUP@example.com", "member", "https://example.test"); !errors.Is(err, core.ErrConflict) {
		t.Errorf("duplicate invite = %v, want conflict", err)
	}
}

func TestInviteWithoutMailerDegrades(t *testing.T) {
	ctx := context.Background()
	auth, ix, _ := newMailAuth(t)
	auth.SetMailer(nil)
	if _, _, err := auth.Setup(ctx, "admin", "secret1"); err != nil {
		t.Fatal(err)
	}
	user, err := auth.Invite(ctx, "x@example.com", "member", "https://example.test")
	if !errors.Is(err, service.ErrMailDisabled) {
		t.Fatalf("invite without mailer = %v, want ErrMailDisabled", err)
	}
	// The pending account is persisted so it can be resent later.
	got, _, err := ix.UserByEmail(ctx, "x@example.com")
	if err != nil || got.Status != core.StatusInvited {
		t.Fatalf("pending account missing: %+v (%v)", got, err)
	}
	if user.ID == "" {
		t.Error("returned user should carry the persisted account")
	}
}

func TestRequestPasswordResetSilent(t *testing.T) {
	ctx := context.Background()
	auth, _, cap := newMailAuth(t)
	if _, _, err := auth.Setup(ctx, "admin", "secret1"); err != nil {
		t.Fatal(err)
	}

	if err := auth.RequestPasswordReset(ctx, "nobody@example.com", "https://example.test"); err != nil {
		t.Fatalf("unknown email = %v, want nil", err)
	}
	if cap.len() != 0 {
		t.Fatalf("unknown email must not send: %d", cap.len())
	}
}

func TestPasswordResetInvalidatesSessions(t *testing.T) {
	ctx := context.Background()
	auth, _, cap := newMailAuth(t)
	if _, _, err := auth.Setup(ctx, "admin", "secret1"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Invite(ctx, "bob@example.com", "member", "https://example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.AcceptInvite(ctx, tokenFrom(cap.last()), "secret1"); err != nil {
		t.Fatal(err)
	}
	_, session, err := auth.Login(ctx, "bob@example.com", "secret1")
	if err != nil {
		t.Fatal(err)
	}

	cap.reset()
	if err := auth.RequestPasswordReset(ctx, "BOB@example.com", "https://example.test"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	resetToken := tokenFrom(cap.last())
	if resetToken == "" {
		t.Fatalf("no reset token in mail: %+v", cap.last())
	}

	if err := auth.ResetPassword(ctx, resetToken, "newsecret1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := auth.Authenticate(ctx, session); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("old session should be invalidated, got %v", err)
	}
	if _, _, err := auth.Login(ctx, "bob@example.com", "secret1"); err == nil {
		t.Error("old password should no longer work")
	}
	if _, _, err := auth.Login(ctx, "bob@example.com", "newsecret1"); err != nil {
		t.Errorf("new password login: %v", err)
	}
	if err := auth.ResetPassword(ctx, resetToken, "another1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("reset token reuse = %v, want ErrNotFound", err)
	}
}

func TestVerifyEmailToken(t *testing.T) {
	ctx := context.Background()
	auth, ix, _ := newMailAuth(t)
	now := time.Now().UTC()
	if err := ix.CreateUser(ctx, core.User{
		ID: "u1", Username: "a@example.com", Email: "a@example.com",
		Role: core.RoleMember, Status: core.StatusActive, Created: now, Updated: now,
	}, "hash"); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("vtoken"))
	if err := ix.CreateUserToken(ctx, core.UserToken{
		ID: "t1", UserID: "u1", Purpose: core.PurposeVerify,
		Created: now, Expires: now.Add(time.Hour),
	}, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	user, err := auth.VerifyEmail(ctx, "vtoken")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !user.EmailVerified {
		t.Fatalf("email not verified: %+v", user)
	}
	if _, err := auth.VerifyEmail(ctx, "vtoken"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("verify token reuse = %v, want ErrNotFound", err)
	}
}
