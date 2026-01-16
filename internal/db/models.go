package db

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key with tier-based configuration and rate limiting.
type APIKey struct {
	ID            uuid.UUID  `json:"id"`
	Key           string     `json:"key"`
	Email         *string    `json:"email,omitempty"`
	Tier          string     `json:"tier"` // free|starter|pro|enterprise
	TierExpiresAt *time.Time `json:"tier_expires_at,omitempty"`
	TierUpdatedAt *time.Time `json:"tier_updated_at,omitempty"`
	RateLimitRPS  int        `json:"rate_limit_rps"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
}

// Session represents an execution session with backend mapping and lifecycle tracking.
type Session struct {
	ID           string            `json:"id"` // sess_xxx
	APIKeyID     uuid.UUID         `json:"api_key_id"`
	BackendID    *string           `json:"backend_id,omitempty"`    // Generic backend ID (pod name, machine ID, etc.)
	FlyMachineID *string           `json:"fly_machine_id,omitempty"` // Deprecated: Use BackendID instead
	FlyAppID     *string           `json:"fly_app_id,omitempty"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	SetupHash    *string           `json:"setup_hash,omitempty"`
	Status       string            `json:"status"` // pending|running|stopped|failed
	ExitCode     *int              `json:"exit_code,omitempty"`
	Ports        []Port            `json:"ports,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	EndedAt      *time.Time        `json:"ended_at,omitempty"`
}

// GetBackendID returns the backend-specific ID for this session.
// It prefers BackendID, falling back to FlyMachineID for backward compatibility.
func (s *Session) GetBackendID() string {
	if s.BackendID != nil && *s.BackendID != "" {
		return *s.BackendID
	}
	if s.FlyMachineID != nil {
		return *s.FlyMachineID
	}
	return ""
}

// Port represents a port mapping for a session.
type Port struct {
	Container int    `json:"container"`
	Host      int    `json:"host,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	URL       string `json:"url,omitempty"`
}

// UsageMetric represents daily aggregated usage statistics per API key.
type UsageMetric struct {
	ID         int64     `json:"id"`
	APIKeyID   uuid.UUID `json:"api_key_id"`
	Date       time.Time `json:"date"`
	Executions int       `json:"executions"`
	DurationMs int64     `json:"duration_ms"`
}

// SessionUpdate holds fields that can be updated in a session.
type SessionUpdate struct {
	FlyMachineID *string    `json:"fly_machine_id,omitempty"`
	FlyAppID     *string    `json:"fly_app_id,omitempty"`
	Status       *string    `json:"status,omitempty"`
	ExitCode     *int       `json:"exit_code,omitempty"`
	Ports        []Port     `json:"ports,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

// QuotaRequest represents a request from a user for increased quota.
type QuotaRequest struct {
	ID              int        `json:"id"`
	APIKeyID        *uuid.UUID `json:"api_key_id,omitempty"`
	Email           string     `json:"email"`
	Name            *string    `json:"name,omitempty"`
	Company         *string    `json:"company,omitempty"`
	CurrentTier     *string    `json:"current_tier,omitempty"`
	RequestedLimits *string    `json:"requested_limits,omitempty"`
	Budget          *string    `json:"budget,omitempty"`
	UseCase         *string    `json:"use_case,omitempty"`
	Status          string     `json:"status"`
	Notes           *string    `json:"notes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	RespondedAt     *time.Time `json:"responded_at,omitempty"`
}
