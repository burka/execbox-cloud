package api

import (
	"context"
	"fmt"

	"github.com/burka/execbox-cloud/internal/backend/k8s"
	"github.com/burka/execbox/pkg/execbox"
)

// K8sBackend wraps a Kubernetes backend to implement the Backend interface.
type K8sBackend struct {
	backend *k8s.Backend
}

// NewK8sBackend creates a new Kubernetes backend adapter.
func NewK8sBackend(backend *k8s.Backend) *K8sBackend {
	return &K8sBackend{
		backend: backend,
	}
}

// CreateSession creates a new Kubernetes pod and returns session metadata.
func (b *K8sBackend) CreateSession(ctx context.Context, config *CreateSessionConfig) (*Session, *SessionNetwork, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("config cannot be nil")
	}

	// Convert CreateSessionConfig to execbox.Spec
	spec := execbox.Spec{
		Image:   config.Image,
		Command: config.Command,
		Env:     config.Env,
		WorkDir: config.WorkDir,
		Setup:   config.Setup,
	}

	// Add resources
	if config.Resources != nil {
		spec.Resources = &execbox.Resources{
			CPUMillis: config.Resources.CPUMillis,
			MemoryMB:  config.Resources.MemoryMB,
		}
		// Convert CPUMillis to CPUPower
		if config.Resources.CPUMillis > 0 {
			spec.Resources.CPUPower = float32(config.Resources.CPUMillis) / 1000.0
		}
	}

	// Add network mode
	if config.Network != "" {
		spec.Network = config.Network
	}

	// Add ports
	if len(config.Ports) > 0 {
		spec.Ports = make([]execbox.Port, 0, len(config.Ports))
		for _, port := range config.Ports {
			protocol := port.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			spec.Ports = append(spec.Ports, execbox.Port{
				Container: port.Container,
				Protocol:  protocol,
			})
		}
	}

	// Add files
	if len(config.Files) > 0 {
		spec.BuildFiles = make([]execbox.BuildFile, 0, len(config.Files))
		for _, file := range config.Files {
			spec.BuildFiles = append(spec.BuildFiles, execbox.BuildFile{
				Path:    file.Path,
				Content: file.Content,
			})
		}
	}

	// Create pod via execbox backend
	handle, err := b.backend.Run(ctx, spec)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kubernetes pod: %w", err)
	}

	// Get session info
	info := handle.Info()

	// Build session metadata
	session := &Session{
		BackendID: info.ID,
		Status:    mapExecboxStatus(info.Status),
		Host:      "kubernetes", // Could be cluster name or node name
		CreatedAt: info.CreatedAt,
		ExitCode:  info.ExitCode,
	}

	// Build network info
	var network *SessionNetwork
	if info.Network != nil {
		network = &SessionNetwork{
			Mode:  info.Network.Mode,
			Host:  info.Network.Host,
			Ports: make(map[int]BackendPortInfo),
		}

		for containerPort, portInfo := range info.Network.Ports {
			protocol := portInfo.Protocol
			if protocol == "" {
				protocol = "tcp"
			}

			network.Ports[containerPort] = BackendPortInfo{
				Container: containerPort,
				HostPort:  portInfo.HostPort,
				Protocol:  protocol,
				URL:       portInfo.URL,
			}
		}
	}

	return session, network, nil
}

// GetSession retrieves session information for a Kubernetes pod.
func (b *K8sBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	info, err := b.backend.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes pod: %w", err)
	}

	session := &Session{
		BackendID: info.ID,
		Status:    mapExecboxStatus(info.Status),
		Host:      "kubernetes",
		CreatedAt: info.CreatedAt,
		ExitCode:  info.ExitCode,
	}

	return session, nil
}

// StopSession gracefully stops a Kubernetes pod.
func (b *K8sBackend) StopSession(ctx context.Context, sessionID string) error {
	if err := b.backend.Stop(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to stop kubernetes pod: %w", err)
	}
	return nil
}

// DestroySession destroys a Kubernetes pod.
func (b *K8sBackend) DestroySession(ctx context.Context, sessionID string) error {
	if err := b.backend.Destroy(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to destroy kubernetes pod: %w", err)
	}
	return nil
}

// Name returns "kubernetes".
func (b *K8sBackend) Name() string {
	return "kubernetes"
}

// mapExecboxStatus maps execbox.Status to session status strings.
func mapExecboxStatus(status execbox.Status) string {
	switch status {
	case execbox.StatusPending:
		return "pending"
	case execbox.StatusRunning:
		return "running"
	case execbox.StatusStopping:
		return "stopping"
	case execbox.StatusStopped:
		return "stopped"
	case execbox.StatusFailed:
		return "failed"
	case execbox.StatusKilled:
		return "killed"
	default:
		return "pending"
	}
}
