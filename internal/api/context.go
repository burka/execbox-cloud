package api

import (
	"context"

	"github.com/google/uuid"
)

// ctxKey is a type for context keys to avoid collisions
type ctxKey string

const (
	ctxAPIKeyID        ctxKey = "api_key_id"
	ctxAPIKeyRateLimit ctxKey = "api_key_rate_limit"
)

// GetAPIKeyID retrieves the API key ID from the request context.
// Returns the API key ID and true if found, otherwise returns a zero UUID and false.
func GetAPIKeyID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxAPIKeyID).(uuid.UUID)
	return id, ok
}

// WithAPIKeyID adds the API key ID to the request context.
// This is typically called by authentication middleware after validating the API key.
func WithAPIKeyID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxAPIKeyID, id)
}

// GetAPIKeyRateLimit retrieves the rate limit (RPS) from the request context.
// Returns the rate limit and true if found, otherwise returns 0 and false.
func GetAPIKeyRateLimit(ctx context.Context) (int, bool) {
	limit, ok := ctx.Value(ctxAPIKeyRateLimit).(int)
	return limit, ok
}

// WithAPIKeyRateLimit adds the rate limit to the request context.
// This is typically called by authentication middleware after validating the API key.
func WithAPIKeyRateLimit(ctx context.Context, rateLimit int) context.Context {
	return context.WithValue(ctx, ctxAPIKeyRateLimit, rateLimit)
}
