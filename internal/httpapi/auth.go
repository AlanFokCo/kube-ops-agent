package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenManager manages API auth tokens.
type TokenManager struct {
	mu     sync.RWMutex
	tokens map[string]struct{}
}

// NewTokenManager creates from env vars and optional token list.
// Supports API_TOKEN, API_TOKENS (comma-sep), K8SOPS_API_TOKEN.
func NewTokenManager(extraTokens []string) *TokenManager {
	tm := &TokenManager{tokens: make(map[string]struct{})}
	for _, t := range extraTokens {
		if t != "" {
			tm.tokens[t] = struct{}{}
		}
	}
	for _, v := range []string{os.Getenv("API_TOKEN"), os.Getenv("K8SOPS_API_TOKEN")} {
		if v != "" {
			tm.tokens[v] = struct{}{}
		}
	}
	// API_TOKENS supports multiple tokens (comma-separated)
	if v := os.Getenv("API_TOKENS"); v != "" {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tm.tokens[t] = struct{}{}
			}
		}
	}
	return tm
}

// IsValid validates token.
func (tm *TokenManager) IsValid(token string) bool {
	if token == "" {
		return false
	}
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, ok := tm.tokens[token]
	return ok
}

// AddToken adds token (aligned with Python add_token).
func (tm *TokenManager) AddToken(token string) {
	if token == "" {
		return
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tokens[token] = struct{}{}
}

// RemoveToken removes token (aligned with Python remove_token).
func (tm *TokenManager) RemoveToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tokens, token)
}

// RotateToken rotates token (aligned with Python rotate_token).
func (tm *TokenManager) RotateToken(oldToken, newToken string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tokens, oldToken)
	if newToken != "" {
		tm.tokens[newToken] = struct{}{}
	}
}

// TokenCount returns valid token count.
func (tm *TokenManager) TokenCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tokens)
}

// HasTokens returns whether tokens are configured.
func (tm *TokenManager) HasTokens() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tokens) > 0
}

// AuthRateLimiter rate-limits auth failures by IP to prevent brute force.
// Aligned with Python: window_seconds count window, block_seconds block duration.
type AuthRateLimiter struct {
	mu          sync.RWMutex
	failures    map[string]*rateLimitEntry
	blocked     map[string]time.Time
	maxFails    int
	window      time.Duration
	blockWindow time.Duration
}

type rateLimitEntry struct {
	count int
	times []time.Time
}

// NewAuthRateLimiter creates rate limiter.
// maxFails max failures per window, window count window, blockSeconds block duration.
func NewAuthRateLimiter(maxFails int, window time.Duration, blockSeconds int) *AuthRateLimiter {
	if maxFails <= 0 {
		maxFails = 5
	}
	if window <= 0 {
		window = 1 * time.Minute
	}
	if blockSeconds <= 0 {
		blockSeconds = 300
	}
	return &AuthRateLimiter{
		failures:    make(map[string]*rateLimitEntry),
		blocked:     make(map[string]time.Time),
		maxFails:    maxFails,
		window:      window,
		blockWindow: time.Duration(blockSeconds) * time.Second,
	}
}

// RecordFailure records one auth failure.
func (a *AuthRateLimiter) RecordFailure(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	e, ok := a.failures[ip]
	if !ok {
		a.failures[ip] = &rateLimitEntry{count: 1, times: []time.Time{now}}
		return
	}
	// Clean expired entries
	cutoff := now.Add(-a.window)
	var valid []time.Time
	for _, t := range e.times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	e.times = valid
	e.count = len(valid)
	if e.count > a.maxFails {
		a.blocked[ip] = now
	}
}

// IsBlocked checks if IP is blocked.
func (a *AuthRateLimiter) IsBlocked(ip string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if until, ok := a.blocked[ip]; ok {
		if time.Now().Before(until.Add(a.blockWindow)) {
			return true
		}
	}
	return false
}

// defaultExemptPaths returns the standard exempt paths for auth.
func defaultExemptPaths(extra []string) map[string]bool {
	exempt := map[string]bool{
		"/health":      true,
		"/ready":       true,
		"/metrics":     true,
		"/favicon.ico": true,
	}
	for _, p := range extra {
		if p != "" {
			exempt[p] = true
		}
	}
	return exempt
}

// extractToken extracts Bearer token from X-API-Key or Authorization header.
func extractToken(r *http.Request) string {
	if h := r.Header.Get("X-API-Key"); h != "" {
		return h
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// authCheckResult holds the result of an auth check.
type authCheckResult struct {
	OK         bool
	StatusCode int
	Error      string
	Message    string
}

// authCheck performs auth validation. OK true means pass.
func authCheck(r *http.Request, tm *TokenManager, exempt map[string]bool, limiter *AuthRateLimiter) authCheckResult {
	if isExempt(r.URL.Path, exempt) || !tm.HasTokens() {
		return authCheckResult{OK: true}
	}
	ip := clientIP(r)
	if limiter != nil && limiter.IsBlocked(ip) {
		return authCheckResult{StatusCode: http.StatusTooManyRequests, Error: "Too Many Requests", Message: "Too many authentication failures"}
	}
	token := extractToken(r)
	if !tm.IsValid(token) {
		if limiter != nil {
			limiter.RecordFailure(ip)
		}
		return authCheckResult{StatusCode: http.StatusUnauthorized, Error: "Unauthorized", Message: "Invalid or missing authentication token"}
	}
	return authCheckResult{OK: true}
}

// GinAuthMiddleware returns Gin auth middleware. Paths in exemptPaths are not checked.
func GinAuthMiddleware(tm *TokenManager, exemptPaths []string, limiter *AuthRateLimiter) gin.HandlerFunc {
	exempt := defaultExemptPaths(exemptPaths)
	return func(c *gin.Context) {
		res := authCheck(c.Request, tm, exempt, limiter)
		if res.OK {
			c.Next()
			return
		}
		if res.StatusCode == http.StatusUnauthorized {
			c.Header("WWW-Authenticate", "Bearer")
		}
		c.AbortWithStatusJSON(res.StatusCode, gin.H{"error": res.Error, "message": res.Message})
	}
}

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		if i := strings.Index(x, ","); i > 0 {
			return strings.TrimSpace(x[:i])
		}
		return strings.TrimSpace(x)
	}
	if x := r.Header.Get("X-Real-IP"); x != "" {
		return strings.TrimSpace(x)
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// isExempt checks if path is exempt (exact match or * suffix prefix).
func isExempt(path string, exempt map[string]bool) bool {
	if exempt[path] {
		return true
	}
	for p := range exempt {
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if prefix != "" && strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}

// AuthMiddleware returns auth middleware. Paths in exemptPaths are not checked.
// exempt * suffix means prefix match (e.g. /api/*). No limit when limiter is nil.
func AuthMiddleware(next http.Handler, tm *TokenManager, exemptPaths []string, limiter *AuthRateLimiter) http.Handler {
	exempt := defaultExemptPaths(exemptPaths)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := authCheck(r, tm, exempt, limiter)
		if res.OK {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if res.StatusCode == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		w.WriteHeader(res.StatusCode)
		body, _ := json.Marshal(map[string]string{"error": res.Error, "message": res.Message})
		_, _ = w.Write(body)
	})
}
