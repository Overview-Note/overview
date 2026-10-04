package index_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/index"
)

func TestUserEmailFields(t *testing.T) {
	ctx := context.Background()
	ix, err := index.Open(filepath.Join(t.TempDir(), "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()

	now := time.Now().UTC()
	invited := core.User{
		ID: "u1", Username: "alice", Email: "alice@example.com",
		Role: core.RoleMember, Status: core.StatusInvited, Created: now, Updated: now,
	}
	if err := ix.CreateUser(ctx, invited, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, hash, err := ix.UserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("by email: %v", err)
	}
	if got.ID != "u1" || got.Email != "alice@example.com" || got.Status != core.StatusInvited || got.EmailVerified {
		t.Fatalf("unexpected user: %+v", got)
	}
	if hash != "" {
		t.Fatalf("invited password hash = %q, want empty", hash)
	}

	if n, err := ix.CountActiveUsers(ctx); err != nil || n != 0 {
		t.Fatalf("active users = %d (%v), want 0", n, err)
	}

	if err := ix.ActivateUser(ctx, "u1", "bcrypt-hash", true); err != nil {
		t.Fatalf("activate: %v", err)
	}
	active, hash, err := ix.UserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("by email after activate: %v", err)
	}
	if active.Status != core.StatusActive || !active.EmailVerified || hash != "bcrypt-hash" {
		t.Fatalf("unexpected active user: %+v hash=%q", active, hash)
	}
	if n, err := ix.CountActiveUsers(ctx); err != nil || n != 1 {
		t.Fatalf("active users = %d (%v), want 1", n, err)
	}

	if _, _, err := ix.UserByEmail(ctx, "nobody@example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing email err = %v, want ErrNotFound", err)
	}
}

func TestUserTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	ix, err := index.Open(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()

	now := time.Now().UTC()
	if err := ix.CreateUser(ctx, core.User{
		ID: "u1", Username: "alice", Role: core.RoleMember,
		Status: core.StatusActive, Created: now, Updated: now,
	}, "hash"); err != nil {
		t.Fatal(err)
	}

	tok := core.UserToken{ID: "t1", UserID: "u1", Purpose: core.PurposeInvite,
		Created: now, Expires: now.Add(time.Hour)}
	if err := ix.CreateUserToken(ctx, tok, "hash-invite"); err != nil {
		t.Fatalf("create token: %v", err)
	}

	// Wrong purpose is rejected.
	if _, err := ix.ConsumeUserToken(ctx, "hash-invite", core.PurposeReset, now); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("wrong purpose = %v, want ErrNotFound", err)
	}
	// Correct purpose succeeds once.
	got, err := ix.ConsumeUserToken(ctx, "hash-invite", core.PurposeInvite, now)
	if err != nil || got.UserID != "u1" {
		t.Fatalf("consume = %+v (%v)", got, err)
	}
	// Reuse is rejected.
	if _, err := ix.ConsumeUserToken(ctx, "hash-invite", core.PurposeInvite, now); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("reuse = %v, want ErrNotFound", err)
	}

	// Expired token is rejected.
	expired := core.UserToken{ID: "t2", UserID: "u1", Purpose: core.PurposeReset,
		Created: now.Add(-2 * time.Hour), Expires: now.Add(-time.Hour)}
	if err := ix.CreateUserToken(ctx, expired, "hash-expired"); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.ConsumeUserToken(ctx, "hash-expired", core.PurposeReset, now); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expired = %v, want ErrNotFound", err)
	}

	// DeleteUserTokens clears outstanding tokens for the purpose.
	fresh := core.UserToken{ID: "t3", UserID: "u1", Purpose: core.PurposeInvite,
		Created: now, Expires: now.Add(time.Hour)}
	if err := ix.CreateUserToken(ctx, fresh, "hash-fresh"); err != nil {
		t.Fatal(err)
	}
	if err := ix.DeleteUserTokens(ctx, "u1", core.PurposeInvite); err != nil {
		t.Fatalf("delete tokens: %v", err)
	}
	if _, err := ix.ConsumeUserToken(ctx, "hash-fresh", core.PurposeInvite, now); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("after delete = %v, want ErrNotFound", err)
	}
}

func TestDeleteUserRemovesTokens(t *testing.T) {
	ctx := context.Background()
	ix, err := index.Open(filepath.Join(t.TempDir(), "delete.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()

	now := time.Now().UTC()
	if err := ix.CreateUser(ctx, core.User{
		ID: "u1", Username: "alice", Role: core.RoleMember,
		Status: core.StatusActive, Created: now, Updated: now,
	}, "hash"); err != nil {
		t.Fatal(err)
	}
	if err := ix.CreateUserToken(ctx, core.UserToken{ID: "t1", UserID: "u1", Purpose: core.PurposeInvite,
		Created: now, Expires: now.Add(time.Hour)}, "hash-token"); err != nil {
		t.Fatal(err)
	}

	if err := ix.DeleteUser(ctx, "u1"); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := ix.ConsumeUserToken(ctx, "hash-token", core.PurposeInvite, now); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("token survived user deletion: %v", err)
	}
}

func TestMigrationIdempotentAndBackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reopen.db")
	ctx := context.Background()
	now := time.Now().UTC()

	ix, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.CreateUser(ctx, core.User{
		ID: "legacy", Username: "legacy", Role: core.RoleAdmin,
		Created: now, Updated: now,
	}, "hash"); err != nil {
		t.Fatal(err)
	}
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}

	ix2, err := index.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer ix2.Close()
	legacy, _, err := ix2.UserByUsername(ctx, "legacy")
	if err != nil {
		t.Fatalf("legacy user lost: %v", err)
	}
	if legacy.Status != core.StatusActive || legacy.Email != "" {
		t.Fatalf("legacy user not migrated cleanly: %+v", legacy)
	}
	if n, err := ix2.CountActiveUsers(ctx); err != nil || n != 1 {
		t.Fatalf("active users = %d (%v)", n, err)
	}
}
