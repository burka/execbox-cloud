package db

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key with tier-based configuration and rate limiting.
type APIKey struct {
	ID           uuid.UUID  `json:"id"`
	Key          string     `json:"key"`
	Tier         string     `json:"tier"` // free|pro|enterprise
	RateLimitRPS int        `json:"rate_limit_rps"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// Session represents an execution session with Fly.io machine mapping and lifecycle tracking.
type Session struct {
	ID           string            `json:"id"` // sess_xxx
	APIKeyID     uuid.UUID         `json:"api_key_id"`
	FlyMachineID *string           `json:"fly_machine_id,omitempty"`
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
