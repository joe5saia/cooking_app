package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a small in-memory token bucket rate limiter keyed by an arbitrary string.
//
// This is intentionally process-local: it protects a single API instance and is sufficient
// for local dev and single-instance deployments.
type rateLimiter struct {
	refillPerSecond float64
	burst           float64

	mu        sync.Mutex
	buckets   map[string]*rateBucket
	entryTTL  time.Duration
	nextPrune time.Time
}

type rateBucket struct {
	tokens   float64
	lastTick time.Time
	lastSeen time.Time
}

func (a *App) loginRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.loginLimiter != nil && !a.loginLimiter.allow(clientIPKey(r)) {
			a.audit(r, "auth.login.rate_limited")
			a.writeError(w, r, errRateLimited())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newRateLimiter(perMin, burst int) *rateLimiter {
	if perMin <= 0 || burst <= 0 {
		return nil
	}

	now := time.Now()
	return &rateLimiter{
		refillPerSecond: float64(perMin) / 60.0,
		burst:           float64(burst),
		buckets:         make(map[string]*rateBucket),
		entryTTL:        15 * time.Minute,
		nextPrune:       now.Add(1 * time.Minute),
	}
}

func (l *rateLimiter) allow(key string) bool {
	if l == nil {
		return true
	}

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneLocked(now)

	if strings.TrimSpace(key) == "" {
		key = "*"
	}

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &rateBucket{
			tokens:   l.burst - 1,
			lastTick: now,
			lastSeen: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastTick)
	if elapsed > 0 {
		b.tokens = minFloat(l.burst, b.tokens+elapsed.Seconds()*l.refillPerSecond)
		b.lastTick = now
	}
	b.lastSeen = now

	if b.tokens < 1.0 {
		return false
	}
	b.tokens -= 1.0
	return true
}

func (l *rateLimiter) pruneLocked(now time.Time) {
	if l == nil || l.entryTTL <= 0 {
		return
	}
	if now.Before(l.nextPrune) {
		return
	}
	l.nextPrune = now.Add(1 * time.Minute)

	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.entryTTL {
			delete(l.buckets, k)
		}
	}
}

func clientIPKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if ip := net.ParseIP(host); ip != nil {
		return host
	}
	return host
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
