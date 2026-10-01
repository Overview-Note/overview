package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/overview-app/overview/internal/core"
)

const sessionTTL = 30 * 24 * time.Hour

// AuthService implements account and session management. When the mode is
// "none" authentication is disabled and every request is anonymous.
type AuthService struct {
	users core.UserStore
	mode  string
}

// NewAuth constructs an AuthService. Supported modes: "none", "multi".
func NewAuth(users core.UserStore, mode string) *AuthService {
	if mode != "multi" {
		mode = "none"
	}
	return &AuthService{users: users, mode: mode}
}

// Mode returns the configured authentication mode.
func (a *AuthService) Mode() string { return a.mode }

// Required reports whether requests must be authenticated.
func (a *AuthService) Required() bool { return a.mode == "multi" }

// NeedsSetup reports whether the first admin account must still be created.
func (a *AuthService) NeedsSetup(ctx context.Context) (bool, error) {
	if !a.Required() {
		return false, nil
	}
	n, err := a.users.CountUsers(ctx)
	return n == 0, err
}

// Setup creates the first administrator. It only succeeds while no user exists.
func (a *AuthService) Setup(ctx context.Context, username, password string) (core.User, string, error) {
	if !a.Required() {
		return core.User{}, "", core.Forbiddenf("authentication is disabled")
	}
	n, err := a.users.CountUsers(ctx)
	if err != nil {
		return core.User{}, "", err
	}
	if n > 0 {
		return core.User{}, "", core.Conflictf("already initialized")
	}
	user, err := a.createUser(ctx, username, password, core.RoleAdmin)
	if err != nil {
		return core.User{}, "", err
	}
	token, err := a.newSession(ctx, user.ID)
	return user, token, err
}

// VerifyPassword checks a username/password pair without starting a session.
// Used by non-cookie protocols such as WebDAV Basic auth.
func (a *AuthService) VerifyPassword(ctx context.Context, username, password string) (core.User, error) {
	user, hash, err := a.users.UserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return core.User{}, core.ErrUnauthorized
		}
		return core.User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return core.User{}, core.ErrUnauthorized
	}
	return user, nil
}

// Login verifies credentials and starts a session.
func (a *AuthService) Login(ctx context.Context, username, password string) (core.User, string, error) {
	user, err := a.VerifyPassword(ctx, username, password)
	if err != nil {
		return core.User{}, "", err
	}
	token, err := a.newSession(ctx, user.ID)
	return user, token, err
}

// Authenticate resolves a session token to a user.
func (a *AuthService) Authenticate(ctx context.Context, token string) (core.User, error) {
	return a.users.UserBySession(ctx, token)
}

// Logout destroys a session.
func (a *AuthService) Logout(ctx context.Context, token string) error {
	return a.users.DeleteSession(ctx, token)
}

// ListUsers returns all accounts.
func (a *AuthService) ListUsers(ctx context.Context) ([]core.User, error) {
	return a.users.ListUsers(ctx)
}

// CreateUser creates an account with the given role.
func (a *AuthService) CreateUser(ctx context.Context, username, password, role string) (core.User, error) {
	if role != core.RoleAdmin && role != core.RoleMember {
		role = core.RoleMember
	}
	return a.createUser(ctx, username, password, role)
}

// DeleteUser removes an account, refusing to delete the last administrator.
func (a *AuthService) DeleteUser(ctx context.Context, id string) error {
	users, err := a.users.ListUsers(ctx)
	if err != nil {
		return err
	}
	target, ok := findUser(users, id)
	if !ok {
		return core.ErrNotFound
	}
	if target.IsAdmin() {
		admins := 0
		for _, u := range users {
			if u.IsAdmin() {
				admins++
			}
		}
		if admins <= 1 {
			return core.Invalidf("cannot delete the last administrator")
		}
	}
	return a.users.DeleteUser(ctx, id)
}

// ChangeOwnPassword verifies the current password before updating.
func (a *AuthService) ChangeOwnPassword(ctx context.Context, user core.User, current, next string) error {
	_, hash, err := a.users.UserByUsername(ctx, user.Username)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		return core.ErrUnauthorized
	}
	return a.ChangePassword(ctx, user.ID, next)
}

// ChangePassword updates an account password.
func (a *AuthService) ChangePassword(ctx context.Context, id, password string) error {
	if len(strings.TrimSpace(password)) < 6 {
		return core.Invalidf("password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.users.UpdatePassword(ctx, id, string(hash))
}

func (a *AuthService) createUser(ctx context.Context, username, password, role string) (core.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 2 {
		return core.User{}, core.Invalidf("username must be at least 2 characters")
	}
	if len(password) < 6 {
		return core.User{}, core.Invalidf("password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return core.User{}, err
	}
	now := time.Now().UTC()
	user := core.User{
		ID:       ulid.Make().String(),
		Username: username,
		Role:     role,
		Created:  now,
		Updated:  now,
	}
	if err := a.users.CreateUser(ctx, user, string(hash)); err != nil {
		return core.User{}, err
	}
	return user, nil
}

func (a *AuthService) newSession(ctx context.Context, userID string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if err := a.users.CreateSession(ctx, token, userID, time.Now().UTC().Add(sessionTTL)); err != nil {
		return "", err
	}
	return token, nil
}

func findUser(users []core.User, id string) (core.User, bool) {
	for _, u := range users {
		if u.ID == id {
			return u, true
		}
	}
	return core.User{}, false
}
