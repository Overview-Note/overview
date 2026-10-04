package index

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

var _ core.UserStore = (*Index)(nil)

// CreateUser inserts a new account. An empty status defaults to active.
func (ix *Index) CreateUser(ctx context.Context, u core.User, passwordHash string) error {
	status := u.Status
	if status == "" {
		status = core.StatusActive
	}
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created, updated, email, status, email_verified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, passwordHash, u.Role,
		u.Created.UTC().Format(time.RFC3339), u.Updated.UTC().Format(time.RFC3339),
		u.Email, status, boolToInt(u.EmailVerified))
	if err != nil {
		if isUniqueViolation(err) {
			return core.Conflictf("username or email already exists")
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
		email       sql.NullString
		verified    int
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, username, role, created, updated, password_hash, email, status, email_verified
		FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up, &hash, &email, &u.Status, &verified)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.User{}, "", core.ErrNotFound
		}
		return core.User{}, "", err
	}
	u.Created = parseTime(c)
	u.Updated = parseTime(up)
	applyUser(&u, email, verified)
	return u, hash, nil
}

// UserByEmail returns the account and its password hash. Accounts without an
// email are never matched.
func (ix *Index) UserByEmail(ctx context.Context, email string) (core.User, string, error) {
	var (
		u           core.User
		hash, c, up string
		em          sql.NullString
		verified    int
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, username, role, created, updated, password_hash, email, status, email_verified
		FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up, &hash, &em, &u.Status, &verified)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.User{}, "", core.ErrNotFound
		}
		return core.User{}, "", err
	}
	u.Created = parseTime(c)
	u.Updated = parseTime(up)
	applyUser(&u, em, verified)
	return u, hash, nil
}

// UserByID returns an account by id.
func (ix *Index) UserByID(ctx context.Context, id string) (core.User, error) {
	var (
		u        core.User
		c, up    string
		email    sql.NullString
		verified int
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT id, username, role, created, updated, email, status, email_verified
		FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up, &email, &u.Status, &verified)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.User{}, core.ErrNotFound
		}
		return core.User{}, err
	}
	u.Created = parseTime(c)
	u.Updated = parseTime(up)
	applyUser(&u, email, verified)
	return u, nil
}

// ListUsers returns all accounts ordered by username.
func (ix *Index) ListUsers(ctx context.Context) ([]core.User, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT id, username, role, created, updated, email, status, email_verified
		 FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]core.User, 0, 8)
	for rows.Next() {
		var (
			u        core.User
			c, up    string
			email    sql.NullString
			verified int
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &c, &up, &email, &u.Status, &verified); err != nil {
			return nil, err
		}
		u.Created = parseTime(c)
		u.Updated = parseTime(up)
		applyUser(&u, email, verified)
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser removes an account, its sessions and any outstanding user tokens.
func (ix *Index) DeleteUser(ctx context.Context, id string) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_tokens WHERE user_id = ?`, id); err != nil {
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

// CountActiveUsers returns the number of accounts that are not pending
// activation.
func (ix *Index) CountActiveUsers(ctx context.Context) (int, error) {
	var n int
	err := ix.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status = ?`, core.StatusActive).Scan(&n)
	return n, err
}

// ActivateUser sets the password for a pending account and marks it active.
func (ix *Index) ActivateUser(ctx context.Context, id, passwordHash string, emailVerified bool) error {
	res, err := ix.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, status = ?, email_verified = ?, updated = ?
		WHERE id = ?`,
		passwordHash, core.StatusActive, boolToInt(emailVerified),
		time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.ErrNotFound
	}
	return nil
}

// SetUserStatus updates an account's lifecycle status.
func (ix *Index) SetUserStatus(ctx context.Context, id, status string) error {
	return ix.updateUser(ctx, id, `status = ?`, status)
}

// SetUserEmail sets (or clears) an account's email address.
func (ix *Index) SetUserEmail(ctx context.Context, id, email string) error {
	return ix.updateUser(ctx, id, `email = ?`, email)
}

// SetEmailVerified flags whether an account's email has been verified.
func (ix *Index) SetEmailVerified(ctx context.Context, id string, verified bool) error {
	return ix.updateUser(ctx, id, `email_verified = ?`, boolToInt(verified))
}

func (ix *Index) updateUser(ctx context.Context, id, assignment string, args ...any) error {
	query := `UPDATE users SET ` + assignment + `, updated = ? WHERE id = ?`
	all := append(args, time.Now().UTC().Format(time.RFC3339), id)
	res, err := ix.db.ExecContext(ctx, query, all...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.ErrNotFound
	}
	return nil
}

// CreateUserToken stores a single-use token (hash only).
func (ix *Index) CreateUserToken(ctx context.Context, t core.UserToken, tokenHash string) error {
	_, err := ix.db.ExecContext(ctx, `
		INSERT INTO user_tokens (id, user_id, purpose, token_hash, created, expires, used)
		VALUES (?, ?, ?, ?, ?, ?, 0)`,
		t.ID, t.UserID, string(t.Purpose), tokenHash,
		t.Created.UTC().Format(time.RFC3339), t.Expires.UTC().Format(time.RFC3339))
	return err
}

// ConsumeUserToken atomically validates and burns a single-use token. It
// returns ErrNotFound for a missing, wrong-purpose, already-used or expired
// token. The read-check-write runs inside a transaction; the pool is limited to
// a single connection (see Open), so concurrent consumptions are serialised.
func (ix *Index) ConsumeUserToken(ctx context.Context, tokenHash string, purpose core.UserTokenPurpose, now time.Time) (core.UserToken, error) {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return core.UserToken{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		t            core.UserToken
		p            string
		created, exp string
		used         int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, purpose, created, expires, used
		FROM user_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&t.ID, &t.UserID, &p, &created, &exp, &used)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.UserToken{}, core.ErrNotFound
		}
		return core.UserToken{}, err
	}
	t.Purpose = core.UserTokenPurpose(p)
	t.Created = parseTime(created)
	t.Expires = parseTime(exp)
	if t.Purpose != purpose || used != 0 || !t.Expires.After(now.UTC()) {
		return core.UserToken{}, core.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_tokens SET used = 1 WHERE id = ?`, t.ID); err != nil {
		return core.UserToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.UserToken{}, err
	}
	return t, nil
}

// DeleteUserTokens removes outstanding tokens for a user and purpose.
func (ix *Index) DeleteUserTokens(ctx context.Context, userID string, purpose core.UserTokenPurpose) error {
	_, err := ix.db.ExecContext(ctx,
		`DELETE FROM user_tokens WHERE user_id = ? AND purpose = ?`, userID, string(purpose))
	return err
}

// DeleteExpiredUserTokens prunes tokens that can no longer be redeemed.
func (ix *Index) DeleteExpiredUserTokens(ctx context.Context, now time.Time) error {
	_, err := ix.db.ExecContext(ctx,
		`DELETE FROM user_tokens WHERE expires < ?`, now.UTC().Format(time.RFC3339))
	return err
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
		u        core.User
		c, up    string
		exp      string
		email    sql.NullString
		verified int
	)
	err := ix.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.created, u.updated, u.email, u.status, u.email_verified, s.expires
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ?`, token).
		Scan(&u.ID, &u.Username, &u.Role, &c, &up, &email, &u.Status, &verified, &exp)
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
	applyUser(&u, email, verified)
	return u, nil
}

// DeleteSessionsByUser invalidates every session belonging to a user, used to
// force a re-login after a password reset.
func (ix *Index) DeleteSessionsByUser(ctx context.Context, userID string) error {
	_, err := ix.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
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

func applyUser(u *core.User, email sql.NullString, verified int) {
	if email.Valid {
		u.Email = email.String
	}
	if u.Status == "" {
		u.Status = core.StatusActive
	}
	u.EmailVerified = verified != 0
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE")
}
