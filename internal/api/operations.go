// Package api defines the huma input/output types for OpenAPI documentation.
package api

// Huma input/output types for API operations.
// These wrap the core types with path parameters, query parameters, and body.

// --- Sessions ---

// CreateSessionInput is the input for POST /v1/sessions.
type CreateSessionInput struct {
	Body CreateSessionRequest
}

// CreateSessionOutput is the output for POST /v1/sessions.
type CreateSessionOutput struct {
	Body CreateSessionResponse
}

// ListSessionsInput is the input for GET /v1/sessions.
type ListSessionsInput struct {
}

// ListSessionsOutput is the output for GET /v1/sessions.
type ListSessionsOutput struct {
	Body ListSessionsResponse
}

// GetSessionInput is the input for GET /v1/sessions/{id}.
type GetSessionInput struct {
	ID string `path:"id" doc:"Session ID" example:"sess_abc123" minLength:"1"`
}

// GetSessionOutput is the output for GET /v1/sessions/{id}.
type GetSessionOutput struct {
	Body SessionResponse
}

// StopSessionInput is the input for POST /v1/sessions/{id}/stop.
type StopSessionInput struct {
	ID string `path:"id" doc:"Session ID" example:"sess_abc123" minLength:"1"`
}

// StopSessionOutput is the output for POST /v1/sessions/{id}/stop (204 No Content).
type StopSessionOutput struct {
	Body StopSessionResponse
}

// KillSessionInput is the input for DELETE /v1/sessions/{id}.
type KillSessionInput struct {
	ID string `path:"id" doc:"Session ID" example:"sess_abc123" minLength:"1"`
}

// KillSessionOutput is the output for DELETE /v1/sessions/{id} (204 No Content).
type KillSessionOutput struct {
}

// AttachSessionInput is the input for GET /v1/sessions/{id}/attach (WebSocket upgrade).
type AttachSessionInput struct {
	ID string `path:"id" doc:"Session ID" example:"sess_abc123" minLength:"1"`
}

// --- Quota ---

// CreateQuotaRequestInput is the input for POST /v1/quota-requests.
type CreateQuotaRequestInput struct {
	Body QuotaRequestRequest
}

// CreateQuotaRequestOutput is the output for POST /v1/quota-requests.
type CreateQuotaRequestOutput struct {
	Body QuotaRequestResponse
}

// --- Account ---

// GetAccountInput is the input for GET /v1/account.
type GetAccountInput struct {
}

// GetAccountOutput is the output for GET /v1/account.
type GetAccountOutput struct {
	Body AccountResponse
}

// GetUsageInput is the input for GET /v1/account/usage.
type GetUsageInput struct {
}

// GetUsageOutput is the output for GET /v1/account/usage.
type GetUsageOutput struct {
	Body UsageResponse
}

// CreateAPIKeyInput is the input for POST /v1/keys.
type CreateAPIKeyInput struct {
	Body CreateKeyRequest
}

// CreateAPIKeyOutput is the output for POST /v1/keys.
type CreateAPIKeyOutput struct {
	Body CreateKeyResponse
}

// --- Health ---

// HealthCheckInput is the input for GET /health.
type HealthCheckInput struct {
}

// HealthCheckOutput is the output for GET /health.
type HealthCheckOutput struct {
	Body struct {
		Status string `json:"status" doc:"Health status" example:"ok"`
	}
}
