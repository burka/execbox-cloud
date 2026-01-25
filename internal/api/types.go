// Package api provides HTTP API types for execbox-cloud
package api

// --- Core Request/Response Types ---

// CreateSessionRequest defines the request body for POST /v1/sessions
type CreateSessionRequest struct {
	Image     string            `json:"image" doc:"Container image (e.g., python:3.11, node:20)" example:"python:3.11" minLength:"1"`
	Setup     []string          `json:"setup,omitempty" doc:"RUN commands to bake into image" example:"pip install requests"`
	Files     []FileSpec        `json:"files,omitempty" doc:"Files to include in image"`
	Command   []string          `json:"command,omitempty" doc:"Command to run in container" example:"python"`
	Env       map[string]string `json:"env,omitempty" doc:"Environment variables"`
	WorkDir   string            `json:"workDir,omitempty" doc:"Working directory" example:"/app" default:"/"`
	Resources *Resources        `json:"resources,omitempty" doc:"Resource limits"`
	Network   string            `json:"network,omitempty" doc:"Network mode: none, outgoing, or exposed" enum:"none,outgoing,exposed" example:"outgoing" default:"outgoing"`
	Ports     []PortSpec        `json:"ports,omitempty" doc:"Ports to expose from container"`
}

// FileSpec defines a file to include in the built image.
// This is the API representation (text/base64 encoding).
// For internal backend use, see SessionFile in backend.go which uses []byte.
type FileSpec struct {
	Path     string `json:"path" doc:"Destination path in container" example:"/app/script.py" minLength:"1"`
	Content  string `json:"content" doc:"File content (text or base64)" example:"print('hello')"`
	Encoding string `json:"encoding,omitempty" doc:"Content encoding: utf8 (default) or base64" enum:"utf8,base64" default:"utf8"`
}

// Resources defines resource limits for a session.
// This type is used both in API requests and backend configurations.
// It consolidates the previous Resources (API) and SessionResources (backend) types.
// Extra fields (computePower, storageMB, timeout) are accepted for CLI compatibility.
type Resources struct {
	CPUMillis    int    `json:"cpuMillis,omitempty" doc:"CPU limit in millicores (1000 = 1 CPU core)" example:"1000" minimum:"100" maximum:"8000"`
	MemoryMB     int    `json:"memoryMB,omitempty" doc:"Memory limit in MB" example:"512" minimum:"128" maximum:"8192"`
	TimeoutMs    int    `json:"timeoutMs,omitempty" doc:"Timeout in milliseconds" example:"60000" minimum:"1000" maximum:"300000"`
	ComputePower int    `json:"computePower,omitempty" doc:"Compute power units (legacy, use cpuMillis)" hidden:"true"`
	StorageMB    int    `json:"storageMB,omitempty" doc:"Storage limit in MB (reserved for future use)" hidden:"true"`
	Timeout      string `json:"timeout,omitempty" doc:"Timeout as duration string (legacy, use timeoutMs)" hidden:"true"`
}

// PortSpec defines a port to expose from the container.
// This type is used both in API requests and backend configurations.
// It consolidates the previous PortSpec (API) and SessionPort (backend) types.
type PortSpec struct {
	Container int    `json:"container" doc:"Container port number" example:"8080" minimum:"1" maximum:"65535"`
	Protocol  string `json:"protocol,omitempty" doc:"Protocol: tcp or udp" enum:"tcp,udp" default:"tcp"`
}

// CreateSessionResponse defines the response body for POST /v1/sessions
type CreateSessionResponse struct {
	ID        string       `json:"id" doc:"Unique session identifier" example:"sess_abc123"`
	Status    string       `json:"status" doc:"Session status" enum:"pending,building,running,stopped,failed" example:"building"`
	CreatedAt string       `json:"createdAt" doc:"Session creation timestamp (RFC3339)" example:"2024-01-15T10:30:00Z"`
	Network   *NetworkInfo `json:"network,omitempty" doc:"Network configuration (if network mode is exposed)"`
}

// NetworkInfo contains network configuration details for a session
type NetworkInfo struct {
	Mode  string              `json:"mode"`  // none|outgoing|exposed
	Host  string              `json:"host"`  // Hostname for accessing ports
	Ports map[string]PortInfo `json:"ports"` // Map of container port to host port info
}

// PortInfo contains details about an exposed port
type PortInfo struct {
	HostPort int    `json:"hostPort"` // Host port number
	URL      string `json:"url"`      // Full URL to access the port
}

// SessionResponse defines the response body for GET /v1/sessions/{id}
type SessionResponse struct {
	ID        string       `json:"id"`
	Status    string       `json:"status"`
	Image     string       `json:"image"`
	CreatedAt string       `json:"createdAt"`
	StartedAt *string      `json:"startedAt,omitempty"`
	EndedAt   *string      `json:"endedAt,omitempty"`
	ExitCode  *int         `json:"exitCode,omitempty"`
	Network   *NetworkInfo `json:"network,omitempty"`
}

// ListSessionsResponse defines the response body for GET /v1/sessions
type ListSessionsResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}

// StopSessionResponse defines the response body for POST /v1/sessions/{id}/stop
type StopSessionResponse struct {
	Status string `json:"status"`
}

// GetURLResponse defines the response body for GET /v1/sessions/{id}/url
type GetURLResponse struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort"`
	URL           string `json:"url"`
	Protocol      string `json:"protocol"`
}

// UploadFileResponse defines the response body for POST /v1/sessions/{id}/files
type UploadFileResponse struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// FileEntry represents a file or directory in a listing
type FileEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	Mode  uint32 `json:"mode"`
}

// ListDirectoryResponse defines the response body for GET /v1/sessions/{id}/files?list=true
type ListDirectoryResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// QuotaRequestRequest defines the request body for POST /v1/quota-requests
type QuotaRequestRequest struct {
	Email           string  `json:"email" doc:"Email address" example:"user@example.com" format:"email" minLength:"1"`
	Name            *string `json:"name,omitempty" doc:"Full name" example:"John Doe"`
	Company         *string `json:"company,omitempty" doc:"Company name" example:"Acme Corp"`
	UseCase         *string `json:"use_case,omitempty" doc:"Description of use case" example:"AI code execution for education"`
	RequestedLimits *string `json:"requested_limits,omitempty" doc:"Requested limits" example:"100 sessions/day"`
	Budget          *string `json:"budget,omitempty" doc:"Budget information" example:"$500/month"`
}

// QuotaRequestResponse defines the response body for POST /v1/quota-requests
type QuotaRequestResponse struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// AccountResponse defines the response body for GET /v1/account
type AccountResponse struct {
	Tier          string  `json:"tier" doc:"Account tier (free, developer, enterprise)" example:"developer"`
	Email         *string `json:"email,omitempty" doc:"Account email address" example:"user@example.com"`
	APIKeyID      string  `json:"api_key_id" doc:"API key identifier" example:"uuid-here"`
	APIKeyPreview string  `json:"api_key_preview" doc:"Masked API key preview" example:"sk_live_...abcd"`
	CreatedAt     string  `json:"created_at" doc:"Account creation timestamp (RFC3339)" example:"2024-01-15T10:30:00Z"`
	TierExpiresAt *string `json:"tier_expires_at,omitempty" doc:"Tier expiration timestamp (RFC3339)" example:"2025-01-15T10:30:00Z"`
}

// UsageResponse defines the response body for GET /v1/account/usage
type UsageResponse struct {
	SessionsToday      int    `json:"sessions_today" doc:"Number of sessions created today" example:"42"`
	ActiveSessions     int    `json:"active_sessions" doc:"Number of currently running sessions" example:"3"`
	QuotaUsed          int    `json:"quota_used" doc:"Daily quota used" example:"42"`
	QuotaRemaining     int    `json:"quota_remaining" doc:"Daily quota remaining (-1 for unlimited)" example:"58"`
	Tier               string `json:"tier" doc:"Account tier" example:"developer"`
	ConcurrentLimit    int    `json:"concurrent_limit" doc:"Max concurrent sessions (-1 for unlimited)" example:"5"`
	DailyLimit         int    `json:"daily_limit" doc:"Max sessions per day (-1 for unlimited)" example:"100"`
	MaxDurationSeconds int    `json:"max_duration_seconds" doc:"Max session duration in seconds" example:"3600"`
	MaxMemoryMB        int    `json:"max_memory_mb" doc:"Max memory per session in MB" example:"512"`
}

// WaitlistRequest defines the request body for POST /v1/waitlist
type WaitlistRequest struct {
	Email string  `json:"email" doc:"Email address to join the waitlist" example:"user@example.com" format:"email" minLength:"1"`
	Name  *string `json:"name,omitempty" doc:"Optional display name" example:"Jane Developer"`
}

// WaitlistResponse defines the response body for POST /v1/waitlist
type WaitlistResponse struct {
	ID      string `json:"id" doc:"API key identifier" example:"uuid-here"`
	Key     string `json:"key" doc:"Your API key (save this - only shown once)" example:"sk_live_abc123..."`
	Tier    string `json:"tier" doc:"Your tier" example:"free"`
	Message string `json:"message" doc:"Welcome message" example:"Welcome to execbox! Save your API key."`
}

// --- Huma Input/Output Types ---
// These wrap the core types with path parameters, query parameters, and body.

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

// CreateQuotaRequestInput is the input for POST /v1/quota-requests.
type CreateQuotaRequestInput struct {
	Body QuotaRequestRequest
}

// CreateQuotaRequestOutput is the output for POST /v1/quota-requests.
type CreateQuotaRequestOutput struct {
	Body QuotaRequestResponse
}

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

// JoinWaitlistInput is the input for POST /v1/waitlist.
type JoinWaitlistInput struct {
	Body WaitlistRequest
}

// JoinWaitlistOutput is the output for POST /v1/waitlist.
type JoinWaitlistOutput struct {
	Body WaitlistResponse
}

// HealthCheckInput is the input for GET /health.
type HealthCheckInput struct {
}

// HealthCheckOutput is the output for GET /health.
type HealthCheckOutput struct {
	Body struct {
		Status string `json:"status" doc:"Health status" example:"ok"`
	}
}

// --- Enhanced Usage Types ---

// EnhancedUsageResponse defines an enhanced usage response with detailed metrics
type EnhancedUsageResponse struct {
	UsageResponse
	AccountID             string        `json:"account_id" doc:"Account identifier" example:"acc_123456"`
	HourlyUsage           []HourlyUsage `json:"hourly_usage,omitempty" doc:"Hourly usage breakdown for the last 24 hours"`
	DailyHistory          []DayUsage    `json:"daily_history,omitempty" doc:"Daily usage history"`
	CostEstimateCents     int64         `json:"cost_estimate_cents" doc:"Estimated cost in cents" example:"150"`
	MonthlyCostLimitCents *int64        `json:"monthly_cost_limit_cents,omitempty" doc:"Monthly cost limit in cents" example:"10000"`
	AlertThreshold        int           `json:"alert_threshold" doc:"Alert threshold percentage" example:"80"`
}

// HourlyUsage defines usage metrics for a single hour
type HourlyUsage struct {
	Hour       string `json:"hour" doc:"Hour in ISO8601 format" example:"2024-01-15T10:00:00Z"`
	Executions int    `json:"executions" doc:"Number of executions in this hour" example:"42"`
	CostCents  int64  `json:"cost_cents" doc:"Cost in cents for this hour" example:"25"`
	Errors     int    `json:"errors" doc:"Number of errors in this hour" example:"2"`
}

// DayUsage defines usage metrics for a single day
type DayUsage struct {
	Date       string `json:"date" doc:"Date in ISO8601 format" example:"2024-01-15"`
	Executions int    `json:"executions" doc:"Number of executions on this day" example:"125"`
	DurationMs int64  `json:"duration_ms" doc:"Total execution duration in milliseconds" example:"125000"`
	CostCents  int64  `json:"cost_cents" doc:"Cost in cents for this day" example:"75"`
	Errors     int    `json:"errors" doc:"Number of errors on this day" example:"5"`
}

// AccountLimitsResponse defines the response for account limits
type AccountLimitsResponse struct {
	DailyRequestsLimit      int     `json:"daily_requests_limit" doc:"Maximum daily requests" example:"1000"`
	ConcurrentRequestsLimit int     `json:"concurrent_requests_limit" doc:"Maximum concurrent requests" example:"10"`
	MonthlyCostLimitCents   *int64  `json:"monthly_cost_limit_cents,omitempty" doc:"Monthly cost limit in cents" example:"50000"`
	AlertThreshold          int     `json:"alert_threshold" doc:"Alert threshold percentage" example:"85"`
	BillingEmail            *string `json:"billing_email,omitempty" doc:"Billing email address" example:"billing@example.com"`
	Timezone                string  `json:"timezone" doc:"Account timezone" example:"UTC"`
}

// UpdateAccountLimitsRequest defines the request to update account limits
type UpdateAccountLimitsRequest struct {
	DailyRequestsLimit      *int    `json:"daily_requests_limit,omitempty" doc:"Maximum daily requests" example:"2000"`
	ConcurrentRequestsLimit *int    `json:"concurrent_requests_limit,omitempty" doc:"Maximum concurrent requests" example:"20"`
	MonthlyCostLimitCents   *int64  `json:"monthly_cost_limit_cents,omitempty" doc:"Monthly cost limit in cents" example:"100000"`
	AlertThreshold          *int    `json:"alert_threshold,omitempty" doc:"Alert threshold percentage" example:"90"`
	BillingEmail            *string `json:"billing_email,omitempty" doc:"Billing email address" example:"new-billing@example.com"`
	Timezone                *string `json:"timezone,omitempty" doc:"Account timezone" example:"America/New_York"`
}

// GetEnhancedUsageInput is the input for GET /v1/account/enhanced-usage.
type GetEnhancedUsageInput struct {
	Days int `query:"days" doc:"Number of days to include in daily history" example:"7" default:"7" minimum:"1" maximum:"90"`
}

// GetEnhancedUsageOutput is the output for GET /v1/account/enhanced-usage.
type GetEnhancedUsageOutput struct {
	Body EnhancedUsageResponse
}

// GetAccountLimitsInput is the input for GET /v1/account/limits.
type GetAccountLimitsInput struct {
}

// GetAccountLimitsOutput is the output for GET /v1/account/limits.
type GetAccountLimitsOutput struct {
	Body AccountLimitsResponse
}

// UpdateAccountLimitsInput is the input for PUT /v1/account/limits.
type UpdateAccountLimitsInput struct {
	Body UpdateAccountLimitsRequest
}

// UpdateAccountLimitsOutput is the output for PUT /v1/account/limits.
type UpdateAccountLimitsOutput struct {
	Body AccountLimitsResponse
}

// ExportUsageInput is the input for GET /v1/account/usage/export.
type ExportUsageInput struct {
	Days   int    `query:"days" doc:"Number of days to export" example:"30" default:"30" minimum:"1" maximum:"365"`
	Format string `query:"format" doc:"Export format" enum:"json,csv" default:"json"`
}

// ExportUsageOutput is the output for GET /v1/account/usage/export.
type ExportUsageOutput struct {
	Body []DayUsage
}

// --- API Key Management Types ---

// APIKeyResponse represents an API key in responses (without the secret key).
type APIKeyResponse struct {
	ID                    string  `json:"id" doc:"API key identifier" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name                  *string `json:"name,omitempty" doc:"Key name" example:"Production API"`
	Description           *string `json:"description,omitempty" doc:"Key description" example:"Used for CI/CD pipelines"`
	KeyPreview            string  `json:"key_preview" doc:"Masked API key preview" example:"sk_...abcd"`
	IsActive              bool    `json:"is_active" doc:"Whether key is active" example:"true"`
	ExpiresAt             *string `json:"expires_at,omitempty" doc:"Expiration timestamp (RFC3339)" example:"2025-12-31T23:59:59Z"`
	CustomDailyLimit      *int    `json:"custom_daily_limit,omitempty" doc:"Custom daily request limit" example:"1000"`
	CustomConcurrentLimit *int    `json:"custom_concurrent_limit,omitempty" doc:"Custom concurrent request limit" example:"10"`
	CreatedAt             string  `json:"created_at" doc:"Creation timestamp (RFC3339)" example:"2024-01-15T10:30:00Z"`
	LastUsedAt            *string `json:"last_used_at,omitempty" doc:"Last used timestamp (RFC3339)" example:"2024-06-01T08:00:00Z"`
}

// CreateAPIKeyRequest defines the request to create a new API key.
type CreateAPIKeyRequest struct {
	Name                  string  `json:"name" doc:"Key name" example:"Production API" minLength:"1" maxLength:"255"`
	Description           *string `json:"description,omitempty" doc:"Key description" example:"Used for CI/CD pipelines" maxLength:"1000"`
	ExpiresAt             *string `json:"expires_at,omitempty" doc:"Expiration time (RFC3339)" example:"2025-12-31T23:59:59Z"`
	CustomDailyLimit      *int    `json:"custom_daily_limit,omitempty" doc:"Custom daily limit (must be <= account limit)" minimum:"1"`
	CustomConcurrentLimit *int    `json:"custom_concurrent_limit,omitempty" doc:"Custom concurrent limit (must be <= account limit)" minimum:"1"`
}

// CreateAPIKeyResponse returns the full key (only shown once).
type CreateAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key" doc:"Full API key (save this - only shown once)" example:"sk_abc123def456..."`
}

// UpdateAPIKeyRequest defines the request to update an API key.
type UpdateAPIKeyRequest struct {
	Name                  *string `json:"name,omitempty" doc:"Key name" example:"Staging API" maxLength:"255"`
	Description           *string `json:"description,omitempty" doc:"Key description" example:"Updated description" maxLength:"1000"`
	ExpiresAt             *string `json:"expires_at,omitempty" doc:"Expiration time (RFC3339)" example:"2026-12-31T23:59:59Z"`
	CustomDailyLimit      *int    `json:"custom_daily_limit,omitempty" doc:"Custom daily limit" minimum:"1"`
	CustomConcurrentLimit *int    `json:"custom_concurrent_limit,omitempty" doc:"Custom concurrent limit" minimum:"1"`
}

// ListAPIKeysResponse defines the response for listing API keys.
type ListAPIKeysResponse struct {
	Keys []APIKeyResponse `json:"keys" doc:"List of API keys"`
}

// RotateAPIKeyResponse returns the new key after rotation.
type RotateAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key" doc:"New API key (save this - only shown once)" example:"sk_new123abc456..."`
}

// --- API Key Management Huma Input/Output Types ---

// ListAPIKeysInput is the input for GET /v1/account/keys.
type ListAPIKeysInput struct {
}

// ListAPIKeysOutput is the output for GET /v1/account/keys.
type ListAPIKeysOutput struct {
	Body ListAPIKeysResponse
}

// CreateAPIKeyInput is the input for POST /v1/account/keys.
type CreateAPIKeyInput struct {
	Body CreateAPIKeyRequest
}

// CreateAPIKeyOutput is the output for POST /v1/account/keys.
type CreateAPIKeyOutput struct {
	Body CreateAPIKeyResponse
}

// GetAPIKeyInput is the input for GET /v1/account/keys/{id}.
type GetAPIKeyInput struct {
	ID string `path:"id" doc:"API key ID" example:"550e8400-e29b-41d4-a716-446655440000" minLength:"1"`
}

// GetAPIKeyOutput is the output for GET /v1/account/keys/{id}.
type GetAPIKeyOutput struct {
	Body APIKeyResponse
}

// UpdateAPIKeyInput is the input for PUT /v1/account/keys/{id}.
type UpdateAPIKeyInput struct {
	ID   string `path:"id" doc:"API key ID" example:"550e8400-e29b-41d4-a716-446655440000" minLength:"1"`
	Body UpdateAPIKeyRequest
}

// UpdateAPIKeyOutput is the output for PUT /v1/account/keys/{id}.
type UpdateAPIKeyOutput struct {
	Body APIKeyResponse
}

// DeleteAPIKeyInput is the input for DELETE /v1/account/keys/{id}.
type DeleteAPIKeyInput struct {
	ID string `path:"id" doc:"API key ID" example:"550e8400-e29b-41d4-a716-446655440000" minLength:"1"`
}

// DeleteAPIKeyOutput is the output for DELETE /v1/account/keys/{id} (204 No Content).
type DeleteAPIKeyOutput struct {
}

// RotateAPIKeyInput is the input for POST /v1/account/keys/{id}/rotate.
type RotateAPIKeyInput struct {
	ID string `path:"id" doc:"API key ID" example:"550e8400-e29b-41d4-a716-446655440000" minLength:"1"`
}

// RotateAPIKeyOutput is the output for POST /v1/account/keys/{id}/rotate.
type RotateAPIKeyOutput struct {
	Body RotateAPIKeyResponse
}
