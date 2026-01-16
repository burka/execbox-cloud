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
type Resources struct {
	CPUMillis int `json:"cpuMillis,omitempty" doc:"CPU limit in millicores (1000 = 1 CPU core)" example:"1000" minimum:"100" maximum:"8000"`
	MemoryMB  int `json:"memoryMB,omitempty" doc:"Memory limit in MB" example:"512" minimum:"128" maximum:"8192"`
	TimeoutMs int `json:"timeoutMs,omitempty" doc:"Timeout in milliseconds" example:"60000" minimum:"1000" maximum:"300000"`
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

// CreateKeyRequest defines the request body for POST /v1/keys
type CreateKeyRequest struct {
	Email string  `json:"email" doc:"Email address for the API key" example:"user@example.com" format:"email" minLength:"1"`
	Name  *string `json:"name,omitempty" doc:"Optional display name" example:"My API Key"`
}

// CreateKeyResponse defines the response body for POST /v1/keys
type CreateKeyResponse struct {
	ID      string `json:"id" doc:"API key identifier" example:"uuid-here"`
	Key     string `json:"key" doc:"The API key (only shown once)" example:"sk_live_abc123..."`
	Tier    string `json:"tier" doc:"Assigned tier" example:"free"`
	Message string `json:"message" doc:"Response message" example:"API key created successfully"`
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

// CreateAPIKeyInput is the input for POST /v1/keys.
type CreateAPIKeyInput struct {
	Body CreateKeyRequest
}

// CreateAPIKeyOutput is the output for POST /v1/keys.
type CreateAPIKeyOutput struct {
	Body CreateKeyResponse
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
