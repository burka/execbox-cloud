package api

import (
	"context"
	"time"
)

// Session represents a generic execution session across different backends.
// This abstracts common fields from both Fly machines and Kubernetes pods.
type Session struct {
	ID        string    // Unique session identifier (e.g., sess_abc123)
	BackendID string    // Backend-specific ID (Fly machine ID or K8s pod name)
	Status    string    // Session status (pending, running, stopped, etc.)
	Host      string    // Host/region where session is running
	CreatedAt time.Time // When the session was created
	ExitCode  *int      // Exit code if session has completed
}

// SessionNetwork contains network configuration for a session.
type SessionNetwork struct {
	Mode  string                  // Network mode: none, outgoing, exposed
	Host  string                  // Hostname for accessing ports
	Ports map[int]BackendPortInfo // Map of container port to port info
}

// BackendPortInfo contains details about an exposed port (internal backend representation).
type BackendPortInfo struct {
	Container int    // Container port number
	HostPort  int    // Host port number (may differ from container port)
	Protocol  string // Protocol: tcp or udp
	URL       string // Full URL to access the port
}

// CreateSessionConfig contains configuration for creating a new session.
// This maps from the API's CreateSessionRequest to a backend-agnostic format.
type CreateSessionConfig struct {
	// Container configuration
	Image   string            // Container image
	Command []string          // Command to run
	Env     map[string]string // Environment variables
	WorkDir string            // Working directory

	// Resource limits
	Resources *Resources // CPU and memory limits

	// Network configuration
	Network string     // Network mode: none, outgoing, exposed
	Ports   []PortSpec // Ports to expose

	// Advanced configuration
	Setup       []string      // Setup commands to run before main command
	Files       []SessionFile // Files to include in the container
	AutoDestroy bool          // Whether to auto-destroy on completion
}

// SessionFile defines a file to include in the container.
type SessionFile struct {
	Path    string // Destination path in container
	Content []byte // File content
}

// Backend defines the interface for execution backends.
// This abstracts both Fly.io and Kubernetes backends behind a common API.
type Backend interface {
	// CreateSession creates a new execution session and returns session metadata.
	CreateSession(ctx context.Context, config *CreateSessionConfig) (*Session, *SessionNetwork, error)

	// GetSession retrieves session information by ID.
	GetSession(ctx context.Context, sessionID string) (*Session, error)

	// StopSession gracefully stops a running session.
	StopSession(ctx context.Context, sessionID string) error

	// DestroySession permanently destroys a session and releases all resources.
	DestroySession(ctx context.Context, sessionID string) error

	// Name returns the backend name (e.g., "fly", "kubernetes").
	Name() string
}
