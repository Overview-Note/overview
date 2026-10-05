package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// TokenVerifier validates a bearer or Basic token. It mirrors
// mcp.TokenVerifier so callers can share a single implementation.
type TokenVerifier func(ctx context.Context, token string) error

// localGuard confines a server to the local machine. It rejects requests whose
// Host header is not a loopback name (defeating DNS rebinding) and rejects
// cross-origin state-changing requests. Requests without an Origin/Referer
// header are allowed so non-browser clients such as curl, the CLI and MCP keep
// working. Because authentication may be disabled locally, this guard replaces
// the CSRF protection that SameSite cookies would otherwise provide.
func localGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			writeError(w, http.StatusForbidden, "invalid host")
			return
		}
		if isStateChanging(r.Method) && !sameOriginRequest(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// isLocalHost reports whether host (a Host header value, optionally with a
// port) names the loopback interface.
func isLocalHost(host string) bool {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	switch strings.ToLower(strings.Trim(name, "[]")) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

// sameOriginRequest reports whether a state-changing request is either
// same-origin or sent by a non-browser client that omits Origin/Referer.
func sameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return hostsMatch(u.Host, r.Host)
}

func hostsMatch(a, b string) bool {
	ah, ap := splitHostPort(a)
	bh, bp := splitHostPort(b)
	if !strings.EqualFold(ah, bh) {
		return false
	}
	if ap == "" || bp == "" {
		return true
	}
	return ap == bp
}

func splitHostPort(hostport string) (string, string) {
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(h, "[]"), p
	}
	return strings.Trim(hostport, "[]"), ""
}

// RequireToken wraps an unauthenticated handler (WebDAV, when auth is set to
// "none") so that it demands a valid token instead of allowing anonymous
// access. The token is read from an Authorization: Bearer header or the
// password half of HTTP Basic auth, which is what OS WebDAV clients send.
func RequireToken(next http.Handler, verify TokenVerifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			if _, pass, ok := r.BasicAuth(); ok {
				token = pass
			}
		}
		if token == "" || verify == nil || verify(r.Context(), token) != nil {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}
