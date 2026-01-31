package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ConcurrencyLimiter limits the number of concurrent requests
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// NewConcurrencyLimiter creates a new concurrency limiter
func NewConcurrencyLimiter(maxConcurrent int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		sem: make(chan struct{}, maxConcurrent),
	}
}

// Middleware returns a middleware that limits concurrency
func (cl *ConcurrencyLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case cl.sem <- struct{}{}:
			defer func() { <-cl.sem }()
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "Too many concurrent requests", http.StatusTooManyRequests)
		}
	})
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

// getLimiter returns a limiter for the given key (e.g., IP address)
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
		rl.mu.Unlock()
	}

	return limiter
}

// Middleware returns a middleware that limits request rate per IP
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip := r.RemoteAddr

		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CleanupOldLimiters removes limiters that haven't been used recently
func (rl *RateLimiter) CleanupOldLimiters(maxAge time.Duration) {
	ticker := time.NewTicker(maxAge)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// In a production system, you'd track last access time
		// For simplicity, we'll just clear all limiters periodically
		rl.limiters = make(map[string]*rate.Limiter)
		rl.mu.Unlock()
	}
}
