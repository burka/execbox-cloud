package api

import (
	"context"
	"fmt"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
)

// FlyBackend wraps a Fly.io client to implement the Backend interface.
type FlyBackend struct {
	client FlyClient
}

// NewFlyBackend creates a new Fly backend adapter.
func NewFlyBackend(client FlyClient) *FlyBackend {
	return &FlyBackend{
		client: client,
	}
}

// CreateSession creates a new Fly machine and returns session metadata.
func (b *FlyBackend) CreateSession(ctx context.Context, config *CreateSessionConfig) (*Session, *SessionNetwork, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("config cannot be nil")
	}

	// Build Fly machine configuration
	machineConfig := &fly.MachineConfig{
		Image:       config.Image,
		Cmd:         config.Command,
		Env:         config.Env,
		AutoDestroy: config.AutoDestroy,
	}

	// Add resource configuration
	if config.Resources != nil {
		machineConfig.Guest = &fly.Guest{}
		if config.Resources.CPUMillis > 0 {
			// Convert millicores to CPU count (Fly uses CPU count)
			machineConfig.Guest.CPUs = (config.Resources.CPUMillis + 999) / 1000
		}
		if config.Resources.MemoryMB > 0 {
			machineConfig.Guest.MemoryMB = config.Resources.MemoryMB
		}
	}

	// Add service configuration for exposed ports
	if len(config.Ports) > 0 && config.Network == "exposed" {
		services := make([]fly.Service, 0, len(config.Ports))
		for _, port := range config.Ports {
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
		machineConfig.Services = services
	}

	// Create machine
	machine, err := b.client.CreateMachine(ctx, machineConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create fly machine: %w", err)
	}

	// Build session metadata
	session := &Session{
		BackendID: machine.ID,
		Status:    mapFlyState(machine.State),
		Host:      machine.Region,
		CreatedAt: parseTime(machine.CreatedAt),
	}

	// Build network info
	var network *SessionNetwork
	if len(config.Ports) > 0 && config.Network == "exposed" {
		network = &SessionNetwork{
			Mode:  config.Network,
			Host:  fmt.Sprintf("%s.fly.dev", machine.Region),
			Ports: make(map[int]BackendPortInfo),
		}

		for _, port := range config.Ports {
			protocol := port.Protocol
			if protocol == "" {
				protocol = "tcp"
			}

			network.Ports[port.Container] = BackendPortInfo{
				Container: port.Container,
				HostPort:  port.Container,
				Protocol:  protocol,
				URL:       fmt.Sprintf("%s://%s:%d", protocol, network.Host, port.Container),
			}
		}
	}

	return session, network, nil
}

// GetSession retrieves session information for a Fly machine.
// Note: This is not currently implemented for Fly backend since the FlyClient
// interface doesn't expose GetMachine. Session state is tracked in the database.
func (b *FlyBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return nil, fmt.Errorf("GetSession not implemented for Fly backend - use database for session state")
}

// StopSession stops a Fly machine.
func (b *FlyBackend) StopSession(ctx context.Context, sessionID string) error {
	if err := b.client.StopMachine(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to stop fly machine: %w", err)
	}
	return nil
}

// DestroySession destroys a Fly machine.
func (b *FlyBackend) DestroySession(ctx context.Context, sessionID string) error {
	if err := b.client.DestroyMachine(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to destroy fly machine: %w", err)
	}
	return nil
}

// Name returns "fly".
func (b *FlyBackend) Name() string {
	return "fly"
}

// mapFlyState maps Fly machine states to session status strings.
func mapFlyState(state string) string {
	switch state {
	case "created":
		return "pending"
	case "starting":
		return "pending"
	case "started":
		return "running"
	case "stopping":
		return "stopping"
	case "stopped":
		return "stopped"
	case "destroying":
		return "stopping"
	case "destroyed":
		return "killed"
	default:
		return "pending"
	}
}

// parseTime parses a time string and returns a time.Time.
// Returns zero time if parsing fails.
func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	// Try RFC3339 format
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}
