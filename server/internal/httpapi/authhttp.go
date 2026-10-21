package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"f1tele/server/internal/auth"
)

const loginWindow = 5 * time.Minute

type loginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func (l *loginLimiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-loginWindow)
	kept := l.failures[key][:0]
	for _, at := range l.failures[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	l.failures[key] = kept
	return len(kept) >= 5
}

func (l *loginLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.failures[key], now)
}

func (l *loginLimiter) clear(key string) {
	l.mu.Lock()
	delete(l.failures, key)
	l.mu.Unlock()
}

func (a *API) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.originAllowed(r) {
			writeErr(w, r, http.StatusForbidden, "origin_forbidden", "this Origin is not allowed")
			return
		}
		path := r.URL.Path
		public := path == "/api/v1/health" || path == "/api/v1/auth/login" || path == "/api/v1/auth/logout" || path == "/api/v1/auth/status" || !strings.HasPrefix(path, "/api/v1/")
		if public || a.Auth == nil || a.sessionValid(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeErr(w, r, http.StatusUnauthorized, "authentication_required", "log in to continue")
	})
}

func (a *API) originAllowed(r *http.Request) bool {
	if !a.Config.Config().Security.OriginCheck {
		return true
	}
	values := r.Header.Values("Origin")
	if len(values) == 0 {
		return true
	}
	if len(values) != 1 {
		return false
	}
	u, err := url.Parse(values[0])
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func (a *API) sessionValid(r *http.Request) bool {
	c, err := r.Cookie("f1tele_session")
	if err != nil || c.Value == "" {
		return false
	}
	return a.Auth.ValidateSession(c.Value, time.Now())
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if a.Auth == nil {
		writeErr(w, r, http.StatusServiceUnavailable, "authentication_unavailable", "authentication is not configured")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeErr(w, r, http.StatusBadRequest, "bad_request", "password is required")
		return
	}
	key := remoteHost(r)
	now := time.Now()
	limiter := a.loginLimiter
	if limiter.blocked(key, now) {
		writeErr(w, r, http.StatusTooManyRequests, "login_throttled", "too many failed attempts; try again later")
		return
	}
	if strings.TrimSpace(body.Password) == "" || !a.Auth.Verify(body.Password) {
		limiter.fail(key, now)
		writeErr(w, r, http.StatusUnauthorized, "invalid_password", "password is incorrect")
		return
	}
	limiter.clear(key)
	token, expires, err := a.Auth.IssueSession(now)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	http.SetCookie(w, a.Auth.Cookie(token, expires))
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, auth.ClearCookie())
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (a *API) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":   a.Auth != nil && a.sessionValid(r),
		"auth_configured": a.Auth != nil,
		"origin_check":    a.Config.Config().Security.OriginCheck,
	})
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
