package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/overview-app/overview/internal/core"
	"github.com/overview-app/overview/internal/index"
	"github.com/overview-app/overview/internal/service"
)

func openAuth(t *testing.T, mode string) *service.AuthService {
	t.Helper()
	ix, err := index.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return service.NewAuth(ix, mode)
}

func TestAuthLifecycle(t *testing.T) {
	ctx := context.Background()
	auth := openAuth(t, "multi")

	needs, err := auth.NeedsSetup(ctx)
	if err != nil || !needs {
		t.Fatalf("expected setup required, got %v %v", needs, err)
	}

	admin, token, err := auth.Setup(ctx, "admin", "secret1")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !admin.IsAdmin() || token == "" {
		t.Fatalf("unexpected admin: %+v token=%q", admin, token)
	}

	if needs, _ := auth.NeedsSetup(ctx); needs {
		t.Error("setup should no longer be required")
	}
	if _, _, err := auth.Setup(ctx, "second", "secret1"); err == nil {
		t.Error("second setup must fail")
	}

	if _, _, err := auth.Login(ctx, "admin", "wrong"); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("bad password should be unauthorized, got %v", err)
	}

	_, tok, err := auth.Login(ctx, "admin", "secret1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	user, err := auth.Authenticate(ctx, tok)
	if err != nil || user.Username != "admin" {
		t.Errorf("authenticate: %+v %v", user, err)
	}

	if err := auth.Logout(ctx, tok); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := auth.Authenticate(ctx, tok); !errors.Is(err, core.ErrUnauthorized) {
		t.Errorf("session should be invalid after logout, got %v", err)
	}

	bob, err := auth.CreateUser(ctx, "bob", "secret1", "member")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if bob.Role != core.RoleMember {
		t.Errorf("role = %q", bob.Role)
	}

	if err := auth.DeleteUser(ctx, admin.ID); err == nil {
		t.Error("deleting the last admin must fail")
	}
	if err := auth.DeleteUser(ctx, bob.ID); err != nil {
		t.Errorf("delete member: %v", err)
	}
}

func TestAuthNoneMode(t *testing.T) {
	ctx := context.Background()
	auth := openAuth(t, "none")
	if auth.Required() {
		t.Fatal("none mode must not require auth")
	}
	if needs, err := auth.NeedsSetup(ctx); err != nil || needs {
		t.Errorf("none mode setup = %v %v", needs, err)
	}
}
