package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/google/uuid"
)

// testHandler is a simple handler that returns 200 OK
func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}

// TestAuthMiddleware_ValidKey tests authentication with a valid API key
func TestAuthMiddleware_ValidKey(t *testing.T) {
	mock := newMockDB()
	keyID := uuid.New()
	mock.apiKeys["valid-key"] = &db.APIKey{
		ID:           keyID,
		Key:          "valid-key",
		Tier:         "free",
		RateLimitRPS: 10,
		CreatedAt:    time.Now(),
	}

	middleware := AuthMiddleware(mock)
	handler := middleware(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Give the async update time to complete
	time.Sleep(10 * time.Millisecond)

	if calls := mock.lastUsedCalls[keyID]; calls != 1 {
		t.Errorf("expected 1 UpdateAPIKeyLastUsed call, got %d", calls)
	}
}

// TestAuthMiddleware_InvalidKey tests authentication with an invalid API key
func TestAuthMiddleware_InvalidKey(t *testing.T) {
	mock := newMockDB()
	middleware := AuthMiddleware(mock)
	handler := middleware(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestAuthMiddleware_MissingHeader tests authentication without Authorization header
func TestAuthMiddleware_MissingHeader(t *testing.T) {
	mock := newMockDB()
	middleware := AuthMiddleware(mock)
	handler := middleware(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestAuthMiddleware_InvalidFormat tests authentication with invalid format
func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	mock := newMockDB()
	middleware := AuthMiddleware(mock)
	handler := middleware(testHandler())

	testCases := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "valid-key"},
		{"wrong prefix", "Basic valid-key"},
		{"empty key", "Bearer "},
		{"only bearer", "Bearer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Code)
			}
		})
	}
}

// TestAuthMiddleware_ContextValues tests that API key ID and rate limit are set in context
func TestAuthMiddleware_ContextValues(t *testing.T) {
	mock := newMockDB()
	keyID := uuid.New()
	mock.apiKeys["valid-key"] = &db.APIKey{
		ID:           keyID,
		Key:          "valid-key",
		Tier:         "pro",
		RateLimitRPS: 100,
		CreatedAt:    time.Now(),
	}

	middleware := AuthMiddleware(mock)

	// Create a handler that checks context values
	checkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check API key ID
		id, ok := GetAPIKeyID(r.Context())
		if !ok {
			t.Error("API key ID not found in context")
		}
		if id != keyID {
			t.Errorf("expected API key ID %v, got %v", keyID, id)
		}

		// Check rate limit
		rateLimit, ok := GetAPIKeyRateLimit(r.Context())
		if !ok {
			t.Error("rate limit not found in context")
		}
		if rateLimit != 100 {
			t.Errorf("expected rate limit 100, got %d", rateLimit)
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(checkHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestLoggingMiddleware tests that requests are logged
func TestLoggingMiddleware(t *testing.T) {
	handler := LoggingMiddleware(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestRecoveryMiddleware tests panic recovery
func TestRecoveryMiddleware(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := RecoveryMiddleware(panicHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestRecoveryMiddleware_NoPanic tests that normal requests work
func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	handler := RecoveryMiddleware(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestRateLimiter_Allow tests basic rate limiting
func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()
	apiKeyID := uuid.New()

	// First request should be allowed
	if !rl.allow(apiKeyID, 10.0) {
		t.Error("first request should be allowed")
	}

	// Immediate second request should be allowed (bucket has capacity)
	if !rl.allow(apiKeyID, 10.0) {
		t.Error("second request should be allowed")
	}
}

// TestRateLimiter_Exceeded tests rate limit exceeded
func TestRateLimiter_Exceeded(t *testing.T) {
	rl := NewRateLimiter()
	apiKeyID := uuid.New()
	rateLimit := 5.0

	// Consume all tokens
	for i := 0; i < int(rateLimit); i++ {
		if !rl.allow(apiKeyID, rateLimit) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// Next request should be rate limited
	if rl.allow(apiKeyID, rateLimit) {
		t.Error("request should be rate limited")
	}

	// Wait for tokens to refill
	time.Sleep(time.Second)

	// Should be allowed again
	if !rl.allow(apiKeyID, rateLimit) {
		t.Error("request should be allowed after waiting")
	}
}

// TestRateLimiter_DifferentKeys tests that different API keys have separate buckets
func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter()
	key1 := uuid.New()
	key2 := uuid.New()
	rateLimit := 2.0

	// Exhaust key1's bucket
	rl.allow(key1, rateLimit)
	rl.allow(key1, rateLimit)

	// key1 should be rate limited
	if rl.allow(key1, rateLimit) {
		t.Error("key1 should be rate limited")
	}

	// key2 should still work
	if !rl.allow(key2, rateLimit) {
		t.Error("key2 should be allowed")
	}
}

// TestRateLimiter_Middleware tests the rate limiter middleware
func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter()
	apiKeyID := uuid.New()

	// Create a context with API key ID and rate limit
	ctx := WithAPIKeyID(context.Background(), apiKeyID)
	ctx = WithAPIKeyRateLimit(ctx, 5)

	handler := rl.Middleware()(testHandler())

	// Make requests until rate limited
	var lastStatus int
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		lastStatus = w.Code

		if w.Code == http.StatusTooManyRequests {
			break
		}
	}

	// Should eventually get rate limited
	if lastStatus != http.StatusTooManyRequests {
		t.Error("expected to get rate limited")
	}
}

// TestRateLimiter_Middleware_NoAPIKey tests middleware without API key in context
func TestRateLimiter_Middleware_NoAPIKey(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware()(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestRateLimiter_Middleware_NoRateLimit tests middleware without rate limit in context
func TestRateLimiter_Middleware_NoRateLimit(t *testing.T) {
	rl := NewRateLimiter()
	apiKeyID := uuid.New()
	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	handler := rl.Middleware()(testHandler())

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestMiddlewareChain tests multiple middlewares working together
func TestMiddlewareChain(t *testing.T) {
	mock := newMockDB()
	keyID := uuid.New()
	mock.apiKeys["valid-key"] = &db.APIKey{
		ID:           keyID,
		Key:          "valid-key",
		Tier:         "free",
		RateLimitRPS: 10,
		CreatedAt:    time.Now(),
	}

	rl := NewRateLimiter()

	// Chain middlewares: Recovery -> Logging -> Auth -> RateLimit
	handler := RecoveryMiddleware(
		LoggingMiddleware(
			AuthMiddleware(mock)(
				rl.Middleware()(testHandler()),
			),
		),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestContextHelpers tests the context helper functions
func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test API key ID
	_, ok := GetAPIKeyID(ctx)
	if ok {
		t.Error("should not find API key ID in empty context")
	}

	keyID := uuid.New()
	ctx = WithAPIKeyID(ctx, keyID)

	retrievedID, ok := GetAPIKeyID(ctx)
	if !ok {
		t.Error("should find API key ID in context")
	}
	if retrievedID != keyID {
		t.Errorf("expected %v, got %v", keyID, retrievedID)
	}

	// Test rate limit
	_, ok = GetAPIKeyRateLimit(ctx)
	if ok {
		t.Error("should not find rate limit in context without setting it")
	}

	ctx = WithAPIKeyRateLimit(ctx, 100)

	rateLimit, ok := GetAPIKeyRateLimit(ctx)
	if !ok {
		t.Error("should find rate limit in context")
	}
	if rateLimit != 100 {
		t.Errorf("expected 100, got %d", rateLimit)
	}
}
