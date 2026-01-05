package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RateLimiter manages rate limiting using token bucket algorithm
type RateLimiter struct {
	mu        sync.RWMutex
	buckets   map[uuid.UUID]*tokenBucket
	ipBuckets map[string]*tokenBucket
}

// tokenBucket implements the token bucket algorithm for rate limiting
type tokenBucket struct {
	tokens    float64
	lastTime  time.Time
	rateLimit float64 // tokens per second
}

// NewRateLimiter creates a new RateLimiter instance
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets:   make(map[uuid.UUID]*tokenBucket),
		ipBuckets: make(map[string]*tokenBucket),
	}

	// Start cleanup goroutine to remove stale buckets
	go rl.cleanup()

	return rl
}

// cleanup periodically removes stale buckets to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for id, bucket := range rl.buckets {
			// Remove buckets that haven't been used in the last hour
			if now.Sub(bucket.lastTime) > time.Hour {
				delete(rl.buckets, id)
			}
		}
		for ip, bucket := range rl.ipBuckets {
			// Remove buckets that haven't been used in the last hour
			if now.Sub(bucket.lastTime) > time.Hour {
				delete(rl.ipBuckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns a middleware that checks rate limits per API key
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Get API key ID from context
			apiKeyID, ok := GetAPIKeyID(r.Context())
			if !ok {
				// API key ID should have been set by AuthMiddleware
				// If not found, this is a programming error
				slog.Error("API key ID not found in context")
				WriteError(w, ErrInternal, http.StatusInternalServerError, CodeInternal)
				return
			}

			// Get rate limit from context (set by AuthMiddleware)
			rateLimit, ok := GetAPIKeyRateLimit(r.Context())
			if !ok {
				slog.Error("API key rate limit not found in context")
				WriteError(w, ErrInternal, http.StatusInternalServerError, CodeInternal)
				return
			}

			// 2. Check token bucket
			allowed := rl.allow(apiKeyID, float64(rateLimit))

			// 3. If rate limited: return 429 Too Many Requests
			if !allowed {
				w.Header().Set("Retry-After", "1")
				WriteError(w,
					fmt.Errorf("rate limit exceeded: %d requests per second", rateLimit),
					http.StatusTooManyRequests,
					"RATE_LIMIT_EXCEEDED",
				)
				return
			}

			// 5. Call next handler (token already consumed in allow method)
			next.ServeHTTP(w, r)
		})
	}
}

// allow checks if a request is allowed based on the token bucket algorithm
// Returns true if the request is allowed, false if rate limited
func (rl *RateLimiter) allow(apiKeyID uuid.UUID, rateLimit float64) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[apiKeyID]
	if !exists {
		// Create new bucket with full capacity
		bucket = &tokenBucket{
			tokens:    rateLimit, // Start with full bucket
			lastTime:  now,
			rateLimit: rateLimit,
		}
		rl.buckets[apiKeyID] = bucket
	}

	// Update rate limit if changed
	if bucket.rateLimit != rateLimit {
		bucket.rateLimit = rateLimit
	}

	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens += elapsed * bucket.rateLimit

	// Cap at rate limit (bucket capacity = 1 second worth of requests)
	if bucket.tokens > bucket.rateLimit {
		bucket.tokens = bucket.rateLimit
	}

	bucket.lastTime = now

	// Check if we have at least one token
	if bucket.tokens >= 1.0 {
		// 4. Consume token
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// allowIP checks if a request from an IP is allowed based on the token bucket algorithm
// Returns true if the request is allowed, false if rate limited
func (rl *RateLimiter) allowIP(ip string, rateLimit float64) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.ipBuckets[ip]
	if !exists {
		// Create new bucket with full capacity
		bucket = &tokenBucket{
			tokens:    rateLimit, // Start with full bucket
			lastTime:  now,
			rateLimit: rateLimit,
		}
		rl.ipBuckets[ip] = bucket
	}

	// Update rate limit if changed
	if bucket.rateLimit != rateLimit {
		bucket.rateLimit = rateLimit
	}

	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(bucket.lastTime).Seconds()
	bucket.tokens += elapsed * bucket.rateLimit

	// Cap at rate limit (bucket capacity = 1 second worth of requests)
	if bucket.tokens > bucket.rateLimit {
		bucket.tokens = bucket.rateLimit
	}

	bucket.lastTime = now

	// Check if we have at least one token
	if bucket.tokens >= 1.0 {
		// Consume token
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// IPMiddleware creates rate limiting middleware based on client IP
func (rl *RateLimiter) IPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract IP from request
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = strings.Split(forwarded, ",")[0]
			}

			// Check rate limit (10 requests per second per IP)
			if !rl.allowIP(ip, 10) {
				WriteError(w, fmt.Errorf("rate limit exceeded"), http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
