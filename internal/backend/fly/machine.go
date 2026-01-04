package fly

import (
	"context"
	"fmt"
	"time"
)

// MachineConfig represents the configuration for a Fly.io machine
type MachineConfig struct {
	Image       string            `json:"image"`
	Cmd         []string          `json:"cmd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Services    []Service         `json:"services,omitempty"`
	AutoDestroy bool              `json:"auto_destroy,omitempty"`
	Guest       *Guest            `json:"guest,omitempty"`
}

// Service represents a service configuration for a machine
type Service struct {
	Ports        []ServicePort `json:"ports"`
	InternalPort int           `json:"internal_port"`
	Protocol     string        `json:"protocol"`
}

// ServicePort represents a port configuration for a service
type ServicePort struct {
	Port     int      `json:"port"`
	Handlers []string `json:"handlers,omitempty"`
}

// Guest represents the guest configuration (CPU/memory) for a machine
type Guest struct {
	CPUs     int `json:"cpus,omitempty"`
	MemoryMB int `json:"memory_mb,omitempty"`
}

// Machine represents a Fly.io machine
type Machine struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	State     string         `json:"state"`
	Region    string         `json:"region"`
	Config    *MachineConfig `json:"config"`
	CreatedAt string         `json:"created_at"`
}

// createMachineRequest is the request body for creating a machine
type createMachineRequest struct {
	Name   string         `json:"name,omitempty"`
	Region string         `json:"region,omitempty"`
	Config *MachineConfig `json:"config"`
}

// CreateMachine creates a new machine in the specified app
func (c *Client) CreateMachine(ctx context.Context, config *MachineConfig) (*Machine, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	req := createMachineRequest{
		Config: config,
	}

	path := fmt.Sprintf("/apps/%s/machines", c.appName)
	resp, err := c.request(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}

	var machine Machine
	if err := decodeResponse(resp, &machine); err != nil {
		return nil, err
	}

	return &machine, nil
}

// GetMachine retrieves a machine by ID
func (c *Client) GetMachine(ctx context.Context, machineID string) (*Machine, error) {
	if machineID == "" {
		return nil, fmt.Errorf("machineID cannot be empty")
	}

	path := fmt.Sprintf("/apps/%s/machines/%s", c.appName, machineID)
	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var machine Machine
	if err := decodeResponse(resp, &machine); err != nil {
		return nil, err
	}

	return &machine, nil
}

// StartMachine starts a stopped machine
func (c *Client) StartMachine(ctx context.Context, machineID string) error {
	if machineID == "" {
		return fmt.Errorf("machineID cannot be empty")
	}

	path := fmt.Sprintf("/apps/%s/machines/%s/start", c.appName, machineID)
	resp, err := c.request(ctx, "POST", path, nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// StopMachine stops a running machine
func (c *Client) StopMachine(ctx context.Context, machineID string) error {
	if machineID == "" {
		return fmt.Errorf("machineID cannot be empty")
	}

	path := fmt.Sprintf("/apps/%s/machines/%s/stop", c.appName, machineID)
	resp, err := c.request(ctx, "POST", path, nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// DestroyMachine permanently deletes a machine
func (c *Client) DestroyMachine(ctx context.Context, machineID string) error {
	if machineID == "" {
		return fmt.Errorf("machineID cannot be empty")
	}

	path := fmt.Sprintf("/apps/%s/machines/%s", c.appName, machineID)
	resp, err := c.request(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// WaitForState polls the machine state until it reaches the desired state or timeout
func (c *Client) WaitForState(ctx context.Context, machineID string, desiredState string, timeout time.Duration) error {
	if machineID == "" {
		return fmt.Errorf("machineID cannot be empty")
	}
	if desiredState == "" {
		return fmt.Errorf("desiredState cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for machine to reach state %s: %w", desiredState, ctx.Err())
		case <-ticker.C:
			machine, err := c.GetMachine(ctx, machineID)
			if err != nil {
				return fmt.Errorf("failed to get machine state: %w", err)
			}

			if machine.State == desiredState {
				return nil
			}
		}
	}
}
