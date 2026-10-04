package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

const sessionMaxAge = 30 * 24 * 3600

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAge,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) handleAuthState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needsSetup, err := s.opts.Auth.NeedsSetup(ctx)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	authRequired := s.opts.Auth.Required()
	resp := map[string]any{
		"mode":          s.opts.Auth.Mode(),
		"needsSetup":    needsSetup,
		"authenticated": false,
		"mailEnabled":   s.opts.Mail != nil && s.opts.Mail.Enabled(),
	}
	if s.opts.Site != nil {
		cfg := s.opts.Site.Config()
		resp["registrationEnabled"] = cfg.RegistrationEnabled && authRequired
		resp["loginHint"] = cfg.LoginHint
		resp["loginIcp"] = cfg.LoginICP
		if cfg.LoginLinkURL != "" {
			resp["loginLink"] = map[string]string{
				"text": cfg.LoginLinkText,
				"url":  cfg.LoginLinkURL,
			}
		}
	} else {
		resp["registrationEnabled"] = false
		resp["loginHint"] = ""
		resp["loginIcp"] = ""
	}
	if authRequired {
		if user, err := s.opts.Auth.Authenticate(ctx, s.sessionToken(r)); err == nil {
			resp["authenticated"] = true
			resp["user"] = user
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	user, token, err := s.opts.Auth.Setup(r.Context(), req.Username, req.Password)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	user, token, err := s.opts.Auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := s.sessionToken(r)
	if token != "" {
		_ = s.opts.Auth.Logout(r.Context(), token)
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

type passwordChange struct {
	Current string `json:"current"`
	Next    string `json:"next"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req passwordChange
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := s.opts.Auth.ChangeOwnPassword(r.Context(), user, req.Current, req.Next); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (core.User, bool) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return core.User{}, false
	}
	if !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "administrator privileges required")
		return core.User{}, false
	}
	return user, true
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	users, err := s.opts.Auth.ListUsers(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	user, err := s.opts.Auth.CreateUser(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.opts.Auth.DeleteUser(r.Context(), r.PathValue("id")); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type createTokenRequest struct {
	Name        string `json:"name"`
	ExpiresDays int    `json:"expiresDays"`
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Tokens == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tokens": []any{}})
		return
	}
	tokens, err := s.opts.Tokens.List(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Tokens == nil {
		writeError(w, http.StatusBadRequest, "token management is not enabled")
		return
	}
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	var ttl time.Duration
	if req.ExpiresDays > 0 {
		ttl = time.Duration(req.ExpiresDays) * 24 * time.Hour
	}
	token, secret, err := s.opts.Tokens.Create(r.Context(), req.Name, ttl)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	// The secret is returned exactly once; it is never retrievable again.
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "secret": secret})
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.opts.Tokens == nil {
		writeError(w, http.StatusBadRequest, "token management is not enabled")
		return
	}
	if err := s.opts.Tokens.Revoke(r.Context(), r.PathValue("id")); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

type inviteUserRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// handleInviteUser creates a pending account and emails an invitation.
func (s *Server) handleInviteUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req inviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	user, err := s.opts.Auth.Invite(r.Context(), req.Email, req.Role, s.publicBaseURL(r))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

// handleResendInvite rotates and re-sends an invitation for a pending account.
func (s *Server) handleResendInvite(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.opts.Auth.ResendInvite(r.Context(), r.PathValue("id"), s.publicBaseURL(r)); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type emailRequest struct {
	Email string `json:"email"`
}

// handleSetUserEmail sets or replaces an account's email address (admin only).
// When a verification email could not be sent (mail not configured) the address
// is still saved and verificationSent reports false.
func (s *Server) handleSetUserEmail(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req emailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	user, sent, err := s.opts.Auth.SetEmail(r.Context(), r.PathValue("id"), req.Email, s.publicBaseURL(r))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "verificationSent": sent})
}

// handleSendVerification rotates and re-sends an email verification link for an
// active, unverified account (admin only).
func (s *Server) handleSendVerification(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if err := s.opts.Auth.SendVerification(r.Context(), r.PathValue("id"), s.publicBaseURL(r)); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePasswordRequest always reports success, whether or not the address is
// known, to avoid account enumeration.
func (s *Server) handlePasswordRequest(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !s.rateLimit(w, r, req.Email) {
		return
	}
	if err := s.opts.Auth.RequestPasswordReset(r.Context(), req.Email, s.publicBaseURL(r)); err != nil {
		s.logger.Warn("password reset request failed", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleRegister creates a self-service member account. Registration must be
// enabled in the site settings and authentication must be in multi mode.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !s.opts.Auth.Required() {
		writeError(w, http.StatusForbidden, "authentication is disabled")
		return
	}
	if s.opts.Site == nil || !s.opts.Site.RegistrationEnabled() {
		writeError(w, http.StatusForbidden, "registration is disabled")
		return
	}
	if !s.rateLimit(w, r, req.Email) {
		return
	}
	user, verificationRequired, err := s.opts.Auth.Register(r.Context(), req.Email, req.Password, s.publicBaseURL(r))
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":                 user,
		"verificationRequired": verificationRequired,
	})
}

// handleResendVerification re-sends a verification email. It always reports
// success so the endpoint cannot be used to enumerate accounts.
func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !s.rateLimit(w, r, req.Email) {
		return
	}
	s.opts.Auth.ResendVerification(r.Context(), req.Email, s.publicBaseURL(r))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type tokenPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// handlePasswordReset redeems a reset token. Token failures share a single
// generic message and never distinguish missing, expired or spent tokens.
func (s *Server) handlePasswordReset(w http.ResponseWriter, r *http.Request) {
	var req tokenPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !s.rateLimit(w, r, "") {
		return
	}
	if err := s.opts.Auth.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, core.ErrInvalid) {
			s.writeDomainError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleAcceptInvite activates a pending account using an invitation token.
func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req tokenPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !s.rateLimit(w, r, "") {
		return
	}
	user, err := s.opts.Auth.AcceptInvite(r.Context(), req.Token, req.Password)
	if err != nil {
		if errors.Is(err, core.ErrInvalid) {
			s.writeDomainError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

// handleVerifyEmail redeems an email verification token.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !s.rateLimit(w, r, "") {
		return
	}
	if _, err := s.opts.Auth.VerifyEmail(r.Context(), req.Token); err != nil {
		if errors.Is(err, core.ErrInvalid) {
			s.writeDomainError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
