package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/Overview-Note/overview/internal/core"
)

const sessionTTL = 30 * 24 * time.Hour

// User-token lifetimes.
const (
	inviteTTL = 7 * 24 * time.Hour
	resetTTL  = 2 * time.Hour
	verifyTTL = 24 * time.Hour
)

const minPasswordLength = 6

// ErrEmailNotVerified is returned when credentials are valid but the account's
// email address must be verified before a session can start. Callers map it to
// a 403 with a dedicated code so the client can offer to resend the message.
var ErrEmailNotVerified = errors.New("email is not verified")

// AuthService implements account and session management. When the mode is
// "none" authentication is disabled and every request is anonymous.
type AuthService struct {
	users core.UserStore
	mode  string
	mail  *MailService
}

// SetMailer wires an outbound mail transport for email-based account flows.
// A nil mailer (or one that is not configured) degrades gracefully: invite,
// reset and verification requests report ErrMailDisabled without sending.
func (a *AuthService) SetMailer(m *MailService) { a.mail = m }

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
	n, err := a.users.CountActiveUsers(ctx)
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
	// Accounts with a configured mail transport must confirm their address
	// before signing in. Accounts without an email (legacy or invited) skip it.
	if user.Email != "" && !user.EmailVerified && a.mailReady() {
		return core.User{}, "", ErrEmailNotVerified
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
	// Pending (invited) accounts cannot authenticate, so they do not count
	// towards the last active administrator.
	if target.IsAdmin() && target.Status == core.StatusActive {
		admins := 0
		for _, u := range users {
			if u.IsAdmin() && u.Status == core.StatusActive {
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

// Invite creates a pending account and emails an invitation link. The account
// is persisted before the email is attempted, so a transport failure returns an
// error while leaving the account resendable.
func (a *AuthService) Invite(ctx context.Context, email, role, baseURL string) (core.User, error) {
	if !a.Required() {
		return core.User{}, core.Forbiddenf("authentication is disabled")
	}
	email = normalizeEmail(email)
	if !validEmail(email) {
		return core.User{}, core.Invalidf("invalid email address")
	}
	if role != core.RoleAdmin && role != core.RoleMember {
		role = core.RoleMember
	}
	if _, _, err := a.users.UserByEmail(ctx, email); err == nil {
		return core.User{}, core.Conflictf("email already exists")
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.User{}, err
	}
	if _, _, err := a.users.UserByUsername(ctx, email); err == nil {
		return core.User{}, core.Conflictf("username already exists")
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.User{}, err
	}
	now := time.Now().UTC()
	user := core.User{
		ID:       ulid.Make().String(),
		Username: email,
		Email:    email,
		Role:     role,
		Status:   core.StatusInvited,
		Created:  now,
		Updated:  now,
	}
	if err := a.users.CreateUser(ctx, user, ""); err != nil {
		return core.User{}, err
	}
	token, err := a.issueToken(ctx, user.ID, core.PurposeInvite, inviteTTL)
	if err != nil {
		return core.User{}, err
	}
	if err := a.send(ctx, inviteMessage(user.Email, baseURL, token)); err != nil {
		return user, err
	}
	return user, nil
}

// ResendInvite rotates the invitation token and emails a fresh link.
func (a *AuthService) ResendInvite(ctx context.Context, userID, baseURL string) error {
	if !a.Required() {
		return core.Forbiddenf("authentication is disabled")
	}
	user, err := a.users.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Status != core.StatusInvited {
		return core.Conflictf("user is not pending invitation")
	}
	if user.Email == "" {
		return core.Conflictf("user has no email address")
	}
	token, err := a.issueToken(ctx, user.ID, core.PurposeInvite, inviteTTL)
	if err != nil {
		return err
	}
	return a.send(ctx, inviteMessage(user.Email, baseURL, token))
}

// Register creates a self-service member account. The username is the
// normalised email and the role is always member, so registration can never
// mint an administrator. When mail is configured the account starts unverified
// and a verification message is sent; the boolean reports whether a
// verification step is required before signing in.
func (a *AuthService) Register(ctx context.Context, email, password, baseURL string) (core.User, bool, error) {
	if !a.Required() {
		return core.User{}, false, core.Forbiddenf("authentication is disabled")
	}
	email = normalizeEmail(email)
	if !validEmail(email) {
		return core.User{}, false, core.Invalidf("invalid email address")
	}
	if len(password) < minPasswordLength {
		return core.User{}, false, core.Invalidf("password must be at least %d characters", minPasswordLength)
	}
	if _, _, err := a.users.UserByEmail(ctx, email); err == nil {
		return core.User{}, false, core.Conflictf("email already exists")
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.User{}, false, err
	}
	if _, _, err := a.users.UserByUsername(ctx, email); err == nil {
		return core.User{}, false, core.Conflictf("username already exists")
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.User{}, false, err
	}

	requireVerify := a.mailReady()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return core.User{}, false, err
	}
	now := time.Now().UTC()
	user := core.User{
		ID:            ulid.Make().String(),
		Username:      email,
		Email:         email,
		Role:          core.RoleMember,
		Status:        core.StatusActive,
		EmailVerified: !requireVerify,
		Created:       now,
		Updated:       now,
	}
	if err := a.users.CreateUser(ctx, user, string(hash)); err != nil {
		return core.User{}, false, err
	}
	if !requireVerify {
		return user, false, nil
	}
	token, err := a.issueToken(ctx, user.ID, core.PurposeVerify, verifyTTL)
	if err != nil {
		return user, true, err
	}
	if err := a.send(ctx, verifyMessage(user.Email, baseURL, token)); err != nil {
		return user, true, err
	}
	return user, true, nil
}

// ResendVerification re-issues a verification email for an active, unverified
// account. Unknown, inactive or already-verified addresses are silently
// ignored, so the endpoint cannot be used to enumerate accounts.
func (a *AuthService) ResendVerification(ctx context.Context, email, baseURL string) {
	email = normalizeEmail(email)
	if email == "" || !a.mailReady() {
		return
	}
	user, _, err := a.users.UserByEmail(ctx, email)
	if err != nil {
		return
	}
	if user.Status != core.StatusActive || user.EmailVerified || user.Email == "" {
		return
	}
	token, err := a.issueToken(ctx, user.ID, core.PurposeVerify, verifyTTL)
	if err != nil {
		return
	}
	_ = a.send(ctx, verifyMessage(user.Email, baseURL, token))
}

// RequestPasswordReset emails a reset link. Unknown or inactive accounts are
// silently ignored to prevent enumeration.
func (a *AuthService) RequestPasswordReset(ctx context.Context, email, baseURL string) error {
	email = normalizeEmail(email)
	if email == "" || !a.mailReady() {
		return nil
	}
	user, _, err := a.users.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil
		}
		return err
	}
	if user.Status != core.StatusActive {
		return nil
	}
	token, err := a.issueToken(ctx, user.ID, core.PurposeReset, resetTTL)
	if err != nil {
		return err
	}
	if err := a.send(ctx, resetMessage(user.Email, baseURL, token)); err != nil {
		if errors.Is(err, ErrMailDisabled) {
			return nil
		}
		return err
	}
	return nil
}

// AcceptInvite activates a pending account with a chosen password.
func (a *AuthService) AcceptInvite(ctx context.Context, token, password string) (core.User, error) {
	if len(strings.TrimSpace(password)) < minPasswordLength {
		return core.User{}, core.Invalidf("password must be at least %d characters", minPasswordLength)
	}
	t, err := a.users.ConsumeUserToken(ctx, hashToken(token), core.PurposeInvite, time.Now().UTC())
	if err != nil {
		return core.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return core.User{}, err
	}
	if err := a.users.ActivateUser(ctx, t.UserID, string(hash), true); err != nil {
		return core.User{}, err
	}
	return a.users.UserByID(ctx, t.UserID)
}

// ResetPassword redeems a reset token, updates the password and invalidates all
// sessions so the account must log in again.
func (a *AuthService) ResetPassword(ctx context.Context, token, password string) error {
	if len(strings.TrimSpace(password)) < minPasswordLength {
		return core.Invalidf("password must be at least %d characters", minPasswordLength)
	}
	t, err := a.users.ConsumeUserToken(ctx, hashToken(token), core.PurposeReset, time.Now().UTC())
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := a.users.UpdatePassword(ctx, t.UserID, string(hash)); err != nil {
		return err
	}
	return a.users.DeleteSessionsByUser(ctx, t.UserID)
}

// SetEmail saves (or replaces) an account's email address. It validates and
// normalises the address, rejects addresses owned by another account and resets
// the verification flag whenever the address actually changes. For an active
// account it also sends a fresh verification email; the boolean reports whether
// mail was dispatched, so a missing mail transport does not block the update.
func (a *AuthService) SetEmail(ctx context.Context, userID, email, baseURL string) (core.User, bool, error) {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return core.User{}, false, core.Invalidf("invalid email address")
	}
	user, err := a.users.UserByID(ctx, userID)
	if err != nil {
		return core.User{}, false, err
	}
	if other, _, err := a.users.UserByEmail(ctx, email); err == nil {
		if other.ID != userID {
			return core.User{}, false, core.Conflictf("email already exists")
		}
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.User{}, false, err
	}
	if err := a.users.SetUserEmail(ctx, userID, email); err != nil {
		return core.User{}, false, err
	}
	if normalizeEmail(user.Email) != email {
		if err := a.users.SetEmailVerified(ctx, userID, false); err != nil {
			return core.User{}, false, err
		}
	}
	updated, err := a.users.UserByID(ctx, userID)
	if err != nil {
		return core.User{}, false, err
	}
	if updated.Status != core.StatusActive || !a.mailReady() {
		return updated, false, nil
	}
	token, err := a.issueToken(ctx, userID, core.PurposeVerify, verifyTTL)
	if err != nil {
		return updated, false, err
	}
	if err := a.send(ctx, verifyMessage(updated.Email, baseURL, token)); err != nil {
		return updated, false, nil
	}
	return updated, true, nil
}

// SendVerification issues a verification token for an existing, active account
// with an unverified email and emails it. It reports ErrMailDisabled when no
// transport is configured.
func (a *AuthService) SendVerification(ctx context.Context, userID, baseURL string) error {
	user, err := a.users.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Email == "" {
		return core.Conflictf("user has no email address")
	}
	if user.EmailVerified {
		return core.Conflictf("email is already verified")
	}
	if user.Status != core.StatusActive {
		return core.Conflictf("user is not active")
	}
	if !a.mailReady() {
		return ErrMailDisabled
	}
	token, err := a.issueToken(ctx, userID, core.PurposeVerify, verifyTTL)
	if err != nil {
		return err
	}
	if err := a.send(ctx, verifyMessage(user.Email, baseURL, token)); err != nil {
		if errors.Is(err, ErrMailDisabled) {
			return err
		}
		return ErrMailDelivery
	}
	return nil
}

// VerifyEmail redeems a verification token and flags the account's email as
// verified.
func (a *AuthService) VerifyEmail(ctx context.Context, token string) (core.User, error) {
	t, err := a.users.ConsumeUserToken(ctx, hashToken(token), core.PurposeVerify, time.Now().UTC())
	if err != nil {
		return core.User{}, err
	}
	if err := a.users.SetEmailVerified(ctx, t.UserID, true); err != nil {
		return core.User{}, err
	}
	return a.users.UserByID(ctx, t.UserID)
}

func (a *AuthService) mailReady() bool {
	return a.mail != nil && a.mail.Ready()
}

func (a *AuthService) send(ctx context.Context, msg Message) error {
	if a.mail == nil {
		return ErrMailDisabled
	}
	return a.mail.Send(ctx, msg)
}

func (a *AuthService) issueToken(ctx context.Context, userID string, purpose core.UserTokenPurpose, ttl time.Duration) (string, error) {
	if err := a.users.DeleteUserTokens(ctx, userID, purpose); err != nil {
		return "", err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(buf)
	now := time.Now().UTC()
	t := core.UserToken{
		ID:      ulid.Make().String(),
		UserID:  userID,
		Purpose: purpose,
		Created: now,
		Expires: now.Add(ttl),
	}
	if err := a.users.CreateUserToken(ctx, t, hashToken(secret)); err != nil {
		return "", err
	}
	return secret, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && strings.Contains(email, "@")
}

func findUser(users []core.User, id string) (core.User, bool) {
	for _, u := range users {
		if u.ID == id {
			return u, true
		}
	}
	return core.User{}, false
}
