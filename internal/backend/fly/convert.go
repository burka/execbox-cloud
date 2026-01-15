package fly

import (
	"fmt"
	"time"

	"github.com/burka/execbox/pkg/execbox"
)

// Machine state constants
const (
	MachineStateCreated   = "created"
	MachineStateStarting  = "starting"
	MachineStateStarted   = "started"
	MachineStateStopping  = "stopping"
	MachineStateStopped   = "stopped"
	MachineStateDestroyed = "destroyed"
)

// SpecToMachineConfig converts execbox.Spec to fly.MachineConfig.
func SpecToMachineConfig(spec execbox.Spec, resolvedImage string) *MachineConfig {
	config := &MachineConfig{
		Image:       resolvedImage,
		Env:         spec.Env,
		AutoDestroy: true,
	}

	// Set command if provided
	if len(spec.Command) > 0 {
		config.Cmd = spec.Command
	}

	// Map resources to Guest
	if spec.Resources != nil {
		config.Guest = ResourcesToGuest(spec.Resources)
	}

	// Map ports to Services
	config.Services = PortsToServices(spec)

	return config
}

// ResourcesToGuest maps execbox.Resources to fly.Guest.
func ResourcesToGuest(r *execbox.Resources) *Guest {
	guest := &Guest{}

	// Map CPUPower to CPUs (1.0 -> 1 CPU)
	if r.CPUPower > 0 {
		guest.CPUs = int(r.CPUPower + 0.5)
		if guest.CPUs < 1 {
			guest.CPUs = 1
		}
	} else if r.CPUMillis > 0 {
		guest.CPUs = (r.CPUMillis + 500) / 1000
		if guest.CPUs < 1 {
			guest.CPUs = 1
		}
	}

	// Map MemoryMB directly
	if r.MemoryMB > 0 {
		guest.MemoryMB = r.MemoryMB
	}

	return guest
}

// PortsToServices converts port specs to Fly service configuration.
func PortsToServices(spec execbox.Spec) []Service {
	var ports []execbox.Port
	if spec.Resources != nil && len(spec.Resources.Ports) > 0 {
		ports = spec.Resources.Ports
	} else if len(spec.Ports) > 0 {
		ports = spec.Ports
	}

	if len(ports) == 0 {
		return nil
	}

	var services []Service
	for _, p := range ports {
		protocol := p.Protocol
		if protocol == "" {
			protocol = "tcp"
		}

		handlers := []string{"http"}
		if p.Container == 443 {
			handlers = []string{"http", "tls"}
		}

		service := Service{
			InternalPort: p.Container,
			Protocol:     protocol,
			Ports: []ServicePort{
				{
					Port:     p.Container, // External port same as internal
					Handlers: handlers,
				},
			},
		}
		services = append(services, service)
	}

	return services
}

// MachineToSessionInfo converts fly.Machine to execbox.SessionInfo.
func MachineToSessionInfo(m *Machine, spec execbox.Spec, sessionID string) execbox.SessionInfo {
	info := execbox.SessionInfo{
		ID:          sessionID,
		ContainerID: m.ID,
		Status:      MachineStateToStatus(m.State),
		Spec:        spec,
	}

	if m.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
			info.CreatedAt = t
		}
	}

	info.Network = MachineToNetworkInfo(m)

	return info
}

// MachineStateToStatus maps Fly machine states to execbox.Status.
func MachineStateToStatus(state string) execbox.Status {
	switch state {
	case MachineStateCreated, MachineStateStarting:
		return execbox.StatusPending
	case MachineStateStarted:
		return execbox.StatusRunning
	case MachineStateStopping:
		return execbox.StatusStopping
	case MachineStateStopped:
		return execbox.StatusStopped
	case MachineStateDestroyed:
		return execbox.StatusKilled
	default:
		return execbox.StatusFailed
	}
}

// MachineToNetworkInfo builds NetworkInfo from Machine.
func MachineToNetworkInfo(m *Machine) *execbox.NetworkInfo {
	if m.Config == nil || len(m.Config.Services) == 0 {
		return nil
	}

	info := &execbox.NetworkInfo{
		Mode:  string(execbox.NetworkExposed),
		Host:  fmt.Sprintf("%s.fly.dev", m.Name),
		Ports: make(map[int]execbox.PortInfo),
	}

	for _, svc := range m.Config.Services {
		if len(svc.Ports) > 0 {
			info.Ports[svc.InternalPort] = execbox.PortInfo{
				HostPort: svc.Ports[0].Port,
				Protocol: svc.Protocol,
				URL:      fmt.Sprintf("https://%s.fly.dev", m.Name),
			}
		}
	}

	return info
}
