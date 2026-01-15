package fly

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/burka/execbox/pkg/execbox"
	"github.com/google/uuid"
)

// BackendConfig holds configuration for the Fly backend.
type BackendConfig struct {
	Token       string
	Org         string
	AppName     string
	DefaultWait time.Duration
	BuildCache  BuildCache
}

// Backend implements execbox.Backend for Fly.io machines.
type Backend struct {
	client   *Client
	builder  *Builder
	cache    BuildCache
	appName  string
	handles  map[string]*Handle
	machines map[string]string // sessionID -> machineID
	waitTime time.Duration
	mu       sync.RWMutex
}

// NewBackend creates a new Fly backend.
func NewBackend(cfg BackendConfig) (*Backend, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("fly token required")
	}
	if cfg.AppName == "" {
		return nil, fmt.Errorf("fly app name required")
	}

	waitTime := cfg.DefaultWait
	if waitTime == 0 {
		waitTime = 60 * time.Second
	}

	client := New(cfg.Token, cfg.Org, cfg.AppName)
	builder := NewBuilder(client, cfg.AppName)

	return &Backend{
		client:   client,
		builder:  builder,
		cache:    cfg.BuildCache,
		appName:  cfg.AppName,
		handles:  make(map[string]*Handle),
		machines: make(map[string]string),
		waitTime: waitTime,
	}, nil
}

// Name returns "fly".
func (b *Backend) Name() string {
	return "fly"
}

// Run creates a Fly machine from spec and returns a Handle.
func (b *Backend) Run(ctx context.Context, spec execbox.Spec) (execbox.Handle, error) {
	// Resolve image (use builder if setup commands present)
	resolvedImage := spec.Image
	if len(spec.Setup) > 0 && b.builder != nil && b.cache != nil {
		buildSpec := &BuildSpec{
			BaseImage: spec.Image,
			Setup:     spec.Setup,
		}
		// Add build files if present
		for _, bf := range spec.BuildFiles {
			buildSpec.Files = append(buildSpec.Files, BuildFile{
				Path:    bf.Path,
				Content: bf.Content,
			})
		}

		resolved, err := b.builder.Resolve(ctx, buildSpec, b.cache)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve image: %w", err)
		}
		resolvedImage = resolved
	}

	// Convert spec to machine config
	config := SpecToMachineConfig(spec, resolvedImage)

	// Create machine
	machine, err := b.client.CreateMachine(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine: %w", err)
	}

	// Wait for machine to start
	if err := b.client.WaitForState(ctx, machine.ID, MachineStateStarted, b.waitTime); err != nil {
		// Cleanup on failure
		_ = b.client.DestroyMachine(ctx, machine.ID)
		return nil, fmt.Errorf("machine failed to start: %w", err)
	}

	// Generate session ID
	sessionID := uuid.New().String()

	// Create handle
	handle := NewHandle(b.client, b.appName, machine.ID, sessionID, spec)

	// Register
	b.mu.Lock()
	b.handles[sessionID] = handle
	b.machines[sessionID] = machine.ID
	b.mu.Unlock()

	return handle, nil
}

// Attach reconnects to an existing machine.
func (b *Backend) Attach(ctx context.Context, id string) (execbox.Handle, error) {
	b.mu.RLock()
	h, ok := b.handles[id]
	machineID := b.machines[id]
	b.mu.RUnlock()

	if ok && h != nil {
		return h, nil
	}

	if machineID == "" {
		return nil, execbox.ErrSessionNotFound
	}

	// Verify machine exists
	machine, err := b.client.GetMachine(ctx, machineID)
	if err != nil {
		return nil, execbox.ErrSessionNotFound
	}

	// Recreate handle (we lost the original spec, use minimal)
	spec := execbox.Spec{
		Image: machine.Config.Image,
	}
	if machine.Config.Cmd != nil {
		spec.Command = machine.Config.Cmd
	}

	handle := NewHandle(b.client, b.appName, machineID, id, spec)

	b.mu.Lock()
	b.handles[id] = handle
	b.mu.Unlock()

	return handle, nil
}

// Get returns session info.
func (b *Backend) Get(ctx context.Context, id string) (execbox.SessionInfo, error) {
	b.mu.RLock()
	h, ok := b.handles[id]
	machineID := b.machines[id]
	b.mu.RUnlock()

	if ok && h != nil {
		return h.Info(), nil
	}

	if machineID == "" {
		return execbox.SessionInfo{}, execbox.ErrSessionNotFound
	}

	machine, err := b.client.GetMachine(ctx, machineID)
	if err != nil {
		return execbox.SessionInfo{}, execbox.ErrSessionNotFound
	}

	return MachineToSessionInfo(machine, execbox.Spec{}, id), nil
}

// List returns all sessions matching filter.
func (b *Backend) List(ctx context.Context, filter execbox.Filter) ([]execbox.SessionInfo, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var results []execbox.SessionInfo
	for _, h := range b.handles {
		if h == nil {
			continue
		}
		info := h.Info()

		// Apply status filter
		if filter.Status != nil && info.Status != *filter.Status {
			continue
		}

		// Apply label filter
		if len(filter.Labels) > 0 {
			match := true
			for k, v := range filter.Labels {
				if info.Spec.Labels[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, info)
	}

	return results, nil
}

// Stop gracefully stops a machine.
func (b *Backend) Stop(ctx context.Context, id string) error {
	b.mu.RLock()
	machineID := b.machines[id]
	h := b.handles[id]
	b.mu.RUnlock()

	if machineID == "" {
		return execbox.ErrSessionNotFound
	}

	if h != nil {
		h.SetStopping()
	}

	return b.client.StopMachine(ctx, machineID)
}

// Kill forcefully terminates a machine.
func (b *Backend) Kill(ctx context.Context, id string) error {
	b.mu.RLock()
	machineID := b.machines[id]
	h := b.handles[id]
	b.mu.RUnlock()

	if machineID == "" {
		return execbox.ErrSessionNotFound
	}

	if h != nil {
		h.SetKilled()
	}

	// Fly doesn't have force kill, use stop (it escalates SIGTERM->SIGKILL)
	return b.client.StopMachine(ctx, machineID)
}

// Destroy permanently removes a machine.
func (b *Backend) Destroy(ctx context.Context, id string) error {
	b.mu.Lock()
	machineID := b.machines[id]
	h := b.handles[id]
	delete(b.handles, id)
	delete(b.machines, id)
	b.mu.Unlock()

	if machineID == "" {
		return execbox.ErrSessionNotFound
	}

	if h != nil {
		h.Close()
	}

	return b.client.DestroyMachine(ctx, machineID)
}

// quoteShellArg quotes a single shell argument for safe execution.
// Uses single quotes and escapes any embedded single quotes.
func quoteShellArg(arg string) string {
	// Escape single quotes by replacing ' with '\''
	escaped := strings.ReplaceAll(arg, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

// quoteShellCmd converts a command slice into a safely quoted shell command string.
func quoteShellCmd(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}
	quoted := make([]string, len(cmd))
	for i, arg := range cmd {
		quoted[i] = quoteShellArg(arg)
	}
	return strings.Join(quoted, " ")
}

// Exec runs a command in a running machine.
func (b *Backend) Exec(ctx context.Context, sessionID string, cmd []string) (stdout, stderr string, exitCode int, err error) {
	b.mu.RLock()
	machineID := b.machines[sessionID]
	b.mu.RUnlock()

	if machineID == "" {
		return "", "", -1, execbox.ErrSessionNotFound
	}

	// Fly Exec takes a single command string - use proper shell quoting to prevent injection
	cmdStr := quoteShellCmd(cmd)

	resp, err := b.client.Exec(ctx, machineID, &ExecRequest{
		Cmd: cmdStr,
	})
	if err != nil {
		return "", "", -1, fmt.Errorf("exec failed: %w", err)
	}

	return resp.Stdout, resp.Stderr, int(resp.ExitCode), nil
}

// Health checks if Fly API is reachable.
func (b *Backend) Health(ctx context.Context) error {
	// Try to list machines as a health check
	_, err := b.client.ListMachines(ctx)
	if err != nil {
		return fmt.Errorf("fly API unhealthy: %w", err)
	}
	return nil
}

// Close releases resources.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, h := range b.handles {
		if h != nil {
			h.Close()
		}
	}

	b.handles = make(map[string]*Handle)
	b.machines = make(map[string]string)

	return nil
}
