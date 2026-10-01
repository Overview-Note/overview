package server

import (
	"encoding/json"
	"net/http"

	"github.com/overview-app/overview/internal/core"
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
	resp := map[string]any{
		"mode":          s.opts.Auth.Mode(),
		"needsSetup":    needsSetup,
		"authenticated": false,
	}
	if s.opts.Auth.Required() {
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
