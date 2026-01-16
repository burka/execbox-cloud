package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AuthMiddleware validates Bearer token and sets API key ID in context
func AuthMiddleware(dbClient DBClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
				return
			}

			// 2. Validate "Bearer <key>" format
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
				return
			}

			key := strings.TrimPrefix(authHeader, bearerPrefix)
			if key == "" {
				WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
				return
			}

			// 3. Look up key in database
			apiKey, err := dbClient.GetAPIKeyByKey(r.Context(), key)
			if err != nil {
				// 4. If invalid: return 401 Unauthorized
				if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "not found") {
					WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
					return
				}
				// Database error
				slog.Error("failed to get API key", "error", err)
				WriteError(w, ErrInternal, http.StatusInternalServerError, CodeInternal)
				return
			}

			// 5. Set API key ID, rate limit, and tier in context
			ctx := WithAPIKeyID(r.Context(), apiKey.ID)
			ctx = WithAPIKeyRateLimit(ctx, apiKey.RateLimitRPS)
			ctx = WithAPIKeyTier(ctx, apiKey.Tier)

			// 6. Update last_used_at async (don't block the request)
			go func() {
				// Use detached context - we want the update to complete even if client disconnects
				// This is intentional fire-and-forget behavior for non-critical tracking
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := dbClient.UpdateAPIKeyLastUsed(updateCtx, apiKey.ID); err != nil {
					// Use Warn instead of Error since this is non-critical tracking
					slog.Warn("failed to update API key last used", "error", err, "api_key_id", apiKey.ID)
				}
			}()

			// 7. Call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoggingMiddleware logs requests with slog
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture the status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Log the request with structured fields
		duration := time.Since(start)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
// It also implements http.Hijacker to support WebSocket upgrades.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker for WebSocket support.
// It delegates to the underlying ResponseWriter if it supports hijacking.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				slog.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"method", r.Method,
					"path", r.URL.Path,
				)

				// Return 500 Internal Server Error
				WriteError(w, ErrInternal, http.StatusInternalServerError, CodeInternal)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
