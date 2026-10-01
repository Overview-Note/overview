package index

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/overview-app/overview/internal/core"
)

var _ core.UserStore = (*Index)(nil)

// CreateUser inserts a new account.
func (ix *Index) CreateUser(ctx context.Context, u core.User, passwordHash string) error {
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created, updated)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, passwordHash, u.Role,
		u.Created.UTC().Format(time.RFC3339), u.Updated.UTC().Format(time.RFC3339))
	if err != nil {
		if isUniqueViolation(err) {
			return core.Conflictf("username already exists")
		}
		return err
	}
	return nil
}

// UserByUsername returns the account and its password hash.
func (ix *Index) UserByUsername(ctx context.Context, username string) (core.User, string, error) {
	var (
		u           core.User
		hash, c, up string
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, username, role, created, updated, password_hash
		FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.User{}, "", core.ErrNotFound
		}
		return core.User{}, "", err
	}
	u.Created = parseTime(c)
	u.Updated = parseTime(up)
	return u, hash, nil
}

// UserByID returns an account by id.
func (ix *Index) UserByID(ctx context.Context, id string) (core.User, error) {
	var (
		u     core.User
		c, up string
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, username, role, created, updated FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.User{}, core.ErrNotFound
		}
		return core.User{}, err
	}
	u.Created = parseTime(c)
	u.Updated = parseTime(up)
	return u, nil
}

// ListUsers returns all accounts ordered by username.
func (ix *Index) ListUsers(ctx context.Context) ([]core.User, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, username, role, created, updated FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]core.User, 0, 8)
	for rows.Next() {
		var (
			u     core.User
			c, up string
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &c, &up); err != nil {
			return nil, err
		}
		u.Created = parseTime(c)
		u.Updated = parseTime(up)
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser removes an account and its sessions.
func (ix *Index) DeleteUser(ctx context.Context, id string) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdatePassword changes an account's password hash.
func (ix *Index) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := ix.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated = ? WHERE id = ?`,
		passwordHash, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// CountUsers returns the number of accounts.
func (ix *Index) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := ix.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateSession stores a login session.
func (ix *Index) CreateSession(ctx context.Context, token, userID string, expires time.Time) error {
	_, err := ix.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, created, expires) VALUES (?, ?, ?, ?)`,
		token, userID, time.Now().UTC().Format(time.RFC3339), expires.UTC().Format(time.RFC3339))
	return err
}

// UserBySession resolves a session token to its (non-expired) user.
func (ix *Index) UserBySession(ctx context.Context, token string) (core.User, error) {
	if token == "" {
		return core.User{}, core.ErrUnauthorized
	}
	var (
		u     core.User
		c, up string
		exp   string
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.created, u.updated, s.expires
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ?`, token).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up, &exp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.User{}, core.ErrUnauthorized
		}
		return core.User{}, err
	}
	if !parseTime(exp).After(time.Now().UTC()) {
		_ = ix.DeleteSession(ctx, token)
		return core.User{}, core.ErrUnauthorized
	}
	u.Created = parseTime(c)
	u.Updated = parseTime(up)
	return u, nil
}

// DeleteSession removes a session token.
func (ix *Index) DeleteSession(ctx context.Context, token string) error {
	_, err := ix.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions prunes expired sessions.
func (ix *Index) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := ix.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires < ?`, now.UTC().Format(time.RFC3339))
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE")
}
