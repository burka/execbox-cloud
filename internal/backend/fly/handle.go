package fly

import (
	"fmt"
	"io"
	"sync"

	"github.com/burka/execbox/pkg/execbox"
)

// Handle implements execbox.Handle for Fly.io machines.
// Fly machines don't support interactive stdin/stdout, so these are no-ops.
type Handle struct {
	client    *Client
	appName   string
	machineID string
	sessionID string
	spec      execbox.Spec
	done      chan execbox.ExitResult
	closeOnce sync.Once

	mu     sync.RWMutex
	status execbox.Status
}

// NewHandle creates a new Handle for a Fly machine.
func NewHandle(client *Client, appName, machineID, sessionID string, spec execbox.Spec) *Handle {
	return &Handle{
		client:    client,
		appName:   appName,
		machineID: machineID,
		sessionID: sessionID,
		spec:      spec,
		done:      make(chan execbox.ExitResult, 1),
		status:    execbox.StatusRunning,
	}
}

// ID returns the session ID.
func (h *Handle) ID() string {
	return h.sessionID
}

// Stdin returns a no-op writer (Fly machines don't support stdin).
func (h *Handle) Stdin() io.WriteCloser {
	return &noopWriteCloser{}
}

// Stdout returns a no-op reader (Fly machines don't support stdout streaming).
func (h *Handle) Stdout() io.ReadCloser {
	return &noopReadCloser{}
}

// Stderr returns a no-op reader (Fly machines don't support stderr streaming).
func (h *Handle) Stderr() io.ReadCloser {
	return &noopReadCloser{}
}

// Wait returns a channel that receives the exit result.
func (h *Handle) Wait() <-chan execbox.ExitResult {
	return h.done
}

// Info returns the current session info.
func (h *Handle) Info() execbox.SessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return execbox.SessionInfo{
		ID:          h.sessionID,
		ContainerID: h.machineID,
		Status:      h.status,
		Spec:        h.spec,
		Network:     h.networkInfo(),
	}
}

// URL returns the URL to access a container port.
func (h *Handle) URL(port int) (string, error) {
	// For Fly, machines are exposed via <app-name>.fly.dev
	// We need to get the machine details to build the URL
	return fmt.Sprintf("https://%s.fly.dev", h.appName), nil
}

// SetStopping marks the session as stopping.
func (h *Handle) SetStopping() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status = execbox.StatusStopping
}

// SetKilled marks the session as killed.
func (h *Handle) SetKilled() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status = execbox.StatusKilled
}

// Close releases resources.
func (h *Handle) Close() {
	h.closeOnce.Do(func() {
		close(h.done)
	})
}

// networkInfo builds network info for this handle.
func (h *Handle) networkInfo() *execbox.NetworkInfo {
	if len(h.spec.Ports) == 0 {
		return nil
	}

	info := &execbox.NetworkInfo{
		Mode:  string(execbox.NetworkExposed),
		Host:  fmt.Sprintf("%s.fly.dev", h.appName),
		Ports: make(map[int]execbox.PortInfo),
	}

	for _, p := range h.spec.Ports {
		info.Ports[p.Container] = execbox.PortInfo{
			HostPort: p.Container,
			Protocol: p.Protocol,
			URL:      fmt.Sprintf("https://%s.fly.dev", h.appName),
		}
	}

	return info
}

// noopWriteCloser is a no-op io.WriteCloser.
type noopWriteCloser struct{}

func (n *noopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n *noopWriteCloser) Close() error {
	return nil
}

// noopReadCloser is a no-op io.ReadCloser.
type noopReadCloser struct{}

func (n *noopReadCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (n *noopReadCloser) Close() error {
	return nil
}
