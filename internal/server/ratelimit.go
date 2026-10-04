package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a bounded, in-process fixed-window counter keyed by an opaque
// string. Stale windows are pruned lazily and, when the map is full, the oldest
// entry is evicted so memory stays bounded under abuse.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]*rateWindow
	limit   int
	window  time.Duration
	maxKeys int
}

type rateWindow struct {
	count int
	start time.Time
}

func newRateLimiter(limit int, window time.Duration, maxKeys int) *rateLimiter {
	return &rateLimiter{
		windows: make(map[string]*rateWindow),
		limit:   limit,
		window:  window,
		maxKeys: maxKeys,
	}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	if l == nil || key == "" {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= l.window {
		if !ok && len(l.windows) >= l.maxKeys {
			l.evictLocked()
		}
		l.windows[key] = &rateWindow{count: 1, start: now}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

func (l *rateLimiter) evictLocked() {
	var (
		oldestKey string
		oldest    time.Time
	)
	for k, w := range l.windows {
		if oldestKey == "" || w.start.Before(oldest) {
			oldestKey, oldest = k, w.start
		}
	}
	if oldestKey != "" {
		delete(l.windows, oldestKey)
	}
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// rateLimit enforces the public-endpoint budgets. Every request is counted
// against its client IP; when email is non-empty it is additionally counted
// against the normalised address. It writes 429 and returns false when a limit
// is exceeded.
func (s *Server) rateLimit(w http.ResponseWriter, r *http.Request, email string) bool {
	now := time.Now()
	if !s.ipLimiter.allow("ip:"+clientIP(r), now) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return false
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" && !s.emailLimiter.allow("email:"+email, now) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return false
	}
	return true
}
