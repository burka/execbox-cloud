package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
)

// FlyClient defines the Fly.io operations required by handlers.
// Deprecated: Use Backend interface instead for new code.
type FlyClient interface {
	CreateMachine(ctx context.Context, config *fly.MachineConfig) (*fly.Machine, error)
	StopMachine(ctx context.Context, machineID string) error
	DestroyMachine(ctx context.Context, machineID string) error
}

// ImageBuilder defines the image building operations.
type ImageBuilder interface {
	Resolve(ctx context.Context, spec *fly.BuildSpec, cache fly.BuildCache) (string, error)
}

// Handlers holds dependencies for request handlers.
// This struct is primarily used by WebSocket handlers in websocket.go.
// HTTP handlers have been moved to service types (SessionService, AccountService, QuotaService).
type Handlers struct {
	db      DBClient
	fly     FlyClient // Deprecated: Use backend instead
	backend Backend   // Generic backend (Fly or K8s)
	builder ImageBuilder
	cache   fly.BuildCache
}

// NewHandlers creates a new Handlers instance with the provided database and backend.
func NewHandlers(dbClient DBClient, backend Backend) *Handlers {
	return &Handlers{
		db:      dbClient,
		backend: backend,
	}
}

// NewHandlersWithFly creates a new Handlers instance with the Fly client directly.
// Deprecated: Use NewHandlers with Backend interface instead.
func NewHandlersWithFly(dbClient DBClient, flyClient FlyClient) *Handlers {
	return &Handlers{
		db:  dbClient,
		fly: flyClient,
	}
}

// SetBuilder configures the image builder for handlers that need it.
func (h *Handlers) SetBuilder(builder ImageBuilder, cache fly.BuildCache) {
	h.builder = builder
	h.cache = cache
}

// --- Helper Functions ---
// These functions are used by both service handlers and the old handler tests.

// generateSessionID creates a random session ID with the format "sess_<random>"
func generateSessionID() string {
	return fmt.Sprintf("sess_%s", randHex(12))
}

// randHex generates a random hex string of the specified length
func randHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand failure indicates system compromise or severe misconfiguration
		// We must never fall back to predictable IDs - panic is the only safe response
		panic(fmt.Sprintf("crypto/rand failed: %v - system security compromised", err))
	}
	return hex.EncodeToString(bytes)[:n]
}

// buildMachineConfig creates a Fly machine configuration from a CreateSessionRequest.
// Used by legacy Fly backend code.
func buildMachineConfig(req *CreateSessionRequest) *fly.MachineConfig {
	config := &fly.MachineConfig{
		Image:       req.Image,
		Cmd:         req.Command,
		Env:         req.Env,
		AutoDestroy: false,
	}

	// Add resource configuration if specified
	if req.Resources != nil {
		config.Guest = &fly.Guest{}
		if req.Resources.CPUMillis > 0 {
			// Convert millicores to CPU count (Fly uses CPU count)
			config.Guest.CPUs = (req.Resources.CPUMillis + 999) / 1000
		}
		if req.Resources.MemoryMB > 0 {
			config.Guest.MemoryMB = req.Resources.MemoryMB
		}
	}

	// Add service configuration for exposed ports
	if len(req.Ports) > 0 && req.Network == "exposed" {
		services := make([]fly.Service, 0, len(req.Ports))
		for _, port := range req.Ports {
			protocol := port.Protocol
			if protocol == "" {
				protocol = "tcp"
			}

			services = append(services, fly.Service{
				InternalPort: port.Container,
				Protocol:     protocol,
				Ports: []fly.ServicePort{
					{
						Port: port.Container,
					},
				},
			})
		}
		config.Services = services
	}

	return config
}

// buildCreateSessionConfig converts a CreateSessionRequest to a generic CreateSessionConfig.
func buildCreateSessionConfig(req *CreateSessionRequest, resolvedImage string) *CreateSessionConfig {
	config := &CreateSessionConfig{
		Image:   resolvedImage,
		Command: req.Command,
		Env:     req.Env,
		WorkDir: req.WorkDir,
		Network: req.Network,
		Setup:   req.Setup,
	}

	// Add resources
	if req.Resources != nil {
		config.Resources = &Resources{
			CPUMillis: req.Resources.CPUMillis,
			MemoryMB:  req.Resources.MemoryMB,
			TimeoutMs: req.Resources.TimeoutMs,
		}
	}

	// Add ports
	if len(req.Ports) > 0 {
		config.Ports = make([]PortSpec, 0, len(req.Ports))
		for _, port := range req.Ports {
			protocol := port.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			config.Ports = append(config.Ports, PortSpec{
				Container: port.Container,
				Protocol:  protocol,
			})
		}
	}

	// Add files
	if len(req.Files) > 0 {
		config.Files = make([]SessionFile, 0, len(req.Files))
		for _, file := range req.Files {
			config.Files = append(config.Files, SessionFile{
				Path:    file.Path,
				Content: []byte(file.Content),
			})
		}
	}

	return config
}

// buildPorts converts API PortSpec to database Port models.
func buildPorts(specs []PortSpec) []db.Port {
	if len(specs) == 0 {
		return nil
	}

	ports := make([]db.Port, 0, len(specs))
	for _, spec := range specs {
		protocol := spec.Protocol
		if protocol == "" {
			protocol = "tcp"
		}

		ports = append(ports, db.Port{
			Container: spec.Container,
			Protocol:  protocol,
		})
	}

	return ports
}

// maskAPIKey masks an API key to show only the first 7 and last 4 characters.
func maskAPIKey(key string) string {
	if len(key) < 12 {
		return "sk_****"
	}
	return key[:7] + "..." + key[len(key)-4:]
}

// buildSessionResponse creates a SessionResponse from a database Session.
func buildSessionResponse(session *db.Session) SessionResponse {
	response := SessionResponse{
		ID:        session.ID,
		Status:    session.Status,
		Image:     session.Image,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
		ExitCode:  session.ExitCode,
	}

	if session.StartedAt != nil {
		startedAt := session.StartedAt.Format(time.RFC3339)
		response.StartedAt = &startedAt
	}

	if session.EndedAt != nil {
		endedAt := session.EndedAt.Format(time.RFC3339)
		response.EndedAt = &endedAt
	}

	// Build network info from ports if available
	if len(session.Ports) > 0 {
		portMap := make(map[string]PortInfo)
		for _, port := range session.Ports {
			portKey := fmt.Sprintf("%d", port.Container)
			info := PortInfo{
				HostPort: port.Host,
			}
			if port.URL != "" {
				info.URL = port.URL
			}
			portMap[portKey] = info
		}

		response.Network = &NetworkInfo{
			Mode:  "exposed", // Simplified - would track this in session
			Ports: portMap,
		}
	}

	return response
}
