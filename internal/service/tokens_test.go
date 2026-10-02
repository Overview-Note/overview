package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/service"
)

func TestTokenService(t *testing.T) {
	ix, err := index.Open(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	svc := service.NewTokenService(ix)
	ctx := context.Background()

	tok, secret, err := svc.Create(ctx, "opencode", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if secret == "" || tok.Prefix == "" {
		t.Fatalf("missing secret/prefix: %+v secret=%q", tok, secret)
	}

	// The secret verifies and resolves to the token.
	got, err := svc.Verify(ctx, secret)
	if err != nil || got.ID != tok.ID {
		t.Fatalf("verify = %+v (%v)", got, err)
	}
	// A wrong secret is rejected.
	if _, err := svc.Verify(ctx, "ovk_not-a-real-token"); !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("bad verify = %v, want ErrUnauthorized", err)
	}

	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v (%v)", list, err)
	}

	if err := svc.Revoke(ctx, tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.Verify(ctx, secret); !errors.Is(err, core.ErrUnauthorized) {
		t.Fatalf("verify after revoke = %v, want ErrUnauthorized", err)
	}
}

func TestTokenServiceExpiry(t *testing.T) {
	ix, err := index.Open(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	svc := service.NewTokenService(ix)
	ctx := context.Background()

	// Negative TTL is treated as non-expiring; a positive TTL yields a future
	// expiry that still verifies.
	_, secret, err := svc.Create(ctx, "short", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Verify(ctx, secret); err != nil {
		t.Fatalf("verify fresh: %v", err)
	}
}
