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

// ListMachines returns all machines for the app
func (c *Client) ListMachines(ctx context.Context) ([]Machine, error) {
	path := fmt.Sprintf("/apps/%s/machines", c.appName)
	resp, err := c.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var machines []Machine
	if err := decodeResponse(resp, &machines); err != nil {
		return nil, err
	}

	return machines, nil
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

// ExecRequest represents a request to execute a command in a machine
type ExecRequest struct {
	Cmd     string `json:"cmd"`
	Stdin   string `json:"stdin,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // seconds, default 30
}

// ExecResponse represents the response from executing a command
type ExecResponse struct {
	ExitCode int32  `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// Exec executes a command in a running machine and returns the output
// This is a synchronous operation - it waits for the command to complete
func (c *Client) Exec(ctx context.Context, machineID string, req *ExecRequest) (*ExecResponse, error) {
	if machineID == "" {
		return nil, fmt.Errorf("machineID cannot be empty")
	}
	if req == nil || req.Cmd == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Default timeout to 30 seconds if not specified
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	path := fmt.Sprintf("/apps/%s/machines/%s/exec", c.appName, machineID)
	resp, err := c.request(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}

	var execResp ExecResponse
	if err := decodeResponse(resp, &execResp); err != nil {
		return nil, err
	}

	return &execResp, nil
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
