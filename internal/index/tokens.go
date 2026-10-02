package index

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

var _ core.TokenStore = (*Index)(nil)

// CreateAPIToken stores a new token (hash only).
func (ix *Index) CreateAPIToken(ctx context.Context, t core.APIToken, tokenHash string) error {
	var exp any
	if t.Expires != nil {
		exp = t.Expires.UTC().Format(time.RFC3339)
	}
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO api_tokens (id, name, prefix, token_hash, created, expires)
		VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Prefix, tokenHash, t.Created.UTC().Format(time.RFC3339), exp)
	return err
}

// ListAPITokens returns all tokens, newest first.
func (ix *Index) ListAPITokens(ctx context.Context) ([]core.APIToken, error) {
	rows, err := ix.db.QueryContext(ctx, `
		SELECT id, name, prefix, created, last_used, expires
		FROM api_tokens ORDER BY created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]core.APIToken, 0, 8)
	for rows.Next() {
		var (
			t                 core.APIToken
			created           string
			lastUsed, expires sql.NullString
		)
		if err := rows.Scan(&t.ID, &t.Name, &t.Prefix, &created, &lastUsed, &expires); err != nil {
			return nil, err
		}
		t.Created = parseTime(created)
		if lastUsed.Valid && lastUsed.String != "" {
			v := parseTime(lastUsed.String)
			t.LastUsed = &v
		}
		if expires.Valid && expires.String != "" {
			v := parseTime(expires.String)
			t.Expires = &v
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteAPIToken removes a token by id.
func (ix *Index) DeleteAPIToken(ctx context.Context, id string) error {
	res, err := ix.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.ErrNotFound
	}
	return nil
}

// APITokenByHash resolves a token hash to its metadata.
func (ix *Index) APITokenByHash(ctx context.Context, tokenHash string) (core.APIToken, error) {
	var (
		t                 core.APIToken
		created           string
		lastUsed, expires sql.NullString
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, name, prefix, created, last_used, expires
		FROM api_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&t.ID, &t.Name, &t.Prefix, &created, &lastUsed, &expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.APIToken{}, core.ErrNotFound
		}
		return core.APIToken{}, err
	}
	t.Created = parseTime(created)
	if lastUsed.Valid && lastUsed.String != "" {
		v := parseTime(lastUsed.String)
		t.LastUsed = &v
	}
	if expires.Valid && expires.String != "" {
		v := parseTime(expires.String)
		t.Expires = &v
	}
	return t, nil
}

// TouchAPIToken records the last time a token was used.
func (ix *Index) TouchAPIToken(ctx context.Context, id string, at time.Time) error {
	_, err := ix.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used = ? WHERE id = ?`, at.UTC().Format(time.RFC3339), id)
	return err
}
