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

func TestAPITokenStore(t *testing.T) {
	ix, err := index.Open(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	tok := core.APIToken{ID: "t1", Name: "opencode", Prefix: "ovk_abcd1234", Created: now}
	if err := ix.CreateAPIToken(ctx, tok, "hash1"); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := ix.APITokenByHash(ctx, "hash1")
	if err != nil {
		t.Fatalf("by hash: %v", err)
	}
	if got.ID != "t1" || got.Name != "opencode" || got.LastUsed != nil {
		t.Fatalf("unexpected token: %+v", got)
	}

	if _, err := ix.APITokenByHash(ctx, "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("miss err = %v, want ErrNotFound", err)
	}

	list, err := ix.ListAPITokens(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v (%v)", list, err)
	}

	if err := ix.TouchAPIToken(ctx, "t1", now); err != nil {
		t.Fatalf("touch: %v", err)
	}
	got, _ = ix.APITokenByHash(ctx, "hash1")
	if got.LastUsed == nil {
		t.Fatal("lastUsed not recorded")
	}

	if err := ix.DeleteAPIToken(ctx, "t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := ix.DeleteAPIToken(ctx, "t1"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete missing = %v, want ErrNotFound", err)
	}
}
