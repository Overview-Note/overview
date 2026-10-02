package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/Overview-Note/overview/internal/core"
)

// TokenService manages long-lived API/MCP tokens. Only a hash is persisted; the
// plaintext secret is returned once at creation.
type TokenService struct {
	tokens core.TokenStore
}

// NewTokenService constructs a TokenService over the given store.
func NewTokenService(tokens core.TokenStore) *TokenService {
	return &TokenService{tokens: tokens}
}

const tokenSecretPrefix = "ovk_"

func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Create mints a new token and returns its metadata plus the one-time secret.
// A ttl <= 0 creates a non-expiring token.
func (s *TokenService) Create(ctx context.Context, name string, ttl time.Duration) (core.APIToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return core.APIToken{}, "", core.Invalidf("name is required")
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return core.APIToken{}, "", err
	}
	secret := tokenSecretPrefix + hex.EncodeToString(buf)
	now := time.Now().UTC()
	t := core.APIToken{
		ID:      ulid.Make().String(),
		Name:    name,
		Prefix:  secret[:12], // "ovk_" + 8 hex chars
		Created: now,
	}
	if ttl > 0 {
		exp := now.Add(ttl)
		t.Expires = &exp
	}
	if err := s.tokens.CreateAPIToken(ctx, t, hashToken(secret)); err != nil {
		return core.APIToken{}, "", err
	}
	return t, secret, nil
}

// List returns all tokens (newest first).
func (s *TokenService) List(ctx context.Context) ([]core.APIToken, error) {
	if s == nil || s.tokens == nil {
		return []core.APIToken{}, nil
	}
	return s.tokens.ListAPITokens(ctx)
}

// Revoke permanently deletes a token.
func (s *TokenService) Revoke(ctx context.Context, id string) error {
	return s.tokens.DeleteAPIToken(ctx, id)
}

// Verify resolves a bearer secret to its token, rejecting expired ones and
// recording the last-used time. It returns ErrUnauthorized when unknown.
func (s *TokenService) Verify(ctx context.Context, secret string) (core.APIToken, error) {
	if s == nil || s.tokens == nil || secret == "" {
		return core.APIToken{}, core.ErrUnauthorized
	}
	t, err := s.tokens.APITokenByHash(ctx, hashToken(secret))
	if err != nil {
		return core.APIToken{}, core.ErrUnauthorized
	}
	if t.Expires != nil && !t.Expires.After(time.Now().UTC()) {
		_ = s.tokens.DeleteAPIToken(ctx, t.ID)
		return core.APIToken{}, core.ErrUnauthorized
	}
	_ = s.tokens.TouchAPIToken(ctx, t.ID, time.Now().UTC())
	return t, nil
}
