package k8s

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/burka/execbox/pkg/execbox"
	"github.com/burka/execbox/pkg/pipe"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Handle implements execbox.Handle for Kubernetes pods.
type Handle struct {
	id        string
	podName   string
	namespace string
	spec      execbox.Spec
	status    execbox.Status
	createdAt time.Time
	exitCode  *int
	exitErr   error

	// I/O streams
	stdin  io.WriteCloser
	stdout *pipe.BufferedPipe
	stderr *pipe.BufferedPipe

	// Attach streams from pod
	attachStdout io.ReadCloser
	attachStderr io.ReadCloser

	// Synchronization
	done    chan execbox.ExitResult
	exitSig chan struct{}

	// Port forwarding
	portForwarders map[int]*portForwarder

	// K8s clients (needed for port forward)
	clientset  kubernetes.Interface
	restConfig *rest.Config

	// Network info
	network *execbox.NetworkInfo

	// Watcher cancellation
	cancelWatcher func()

	mu sync.RWMutex
}

// portForwarder manages a single port forward session.
type portForwarder struct {
	localPort  int
	remotePort int
	stopChan   chan struct{}
	readyChan  chan struct{}
	errChan    chan error
}

// NewHandle creates a new Handle for a Kubernetes pod.
func NewHandle(
	id, podName, namespace string,
	spec execbox.Spec,
	clientset kubernetes.Interface,
	restConfig *rest.Config,
) *Handle {
	return &Handle{
		id:             id,
		podName:        podName,
		namespace:      namespace,
		spec:           spec,
		status:         execbox.StatusRunning,
		createdAt:      time.Now(),
		stdout:         pipe.New(),
		stderr:         pipe.New(),
		done:           make(chan execbox.ExitResult, 1),
		exitSig:        make(chan struct{}),
		portForwarders: make(map[int]*portForwarder),
		clientset:      clientset,
		restConfig:     restConfig,
	}
}

// ID returns the session ID.
func (h *Handle) ID() string {
	return h.id
}

// Stdin returns a writer for sending input to the pod.
func (h *Handle) Stdin() io.WriteCloser {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.stdin == nil {
		// Return a no-op writer if stdin is not set up
		return &noopWriteCloser{}
	}
	return h.stdin
}

// Stdout returns a reader for output from the pod.
func (h *Handle) Stdout() io.ReadCloser {
	return h.stdout
}

// Stderr returns a reader for error output from the pod.
func (h *Handle) Stderr() io.ReadCloser {
	return h.stderr
}

// Wait returns a channel that receives the exit result when the session completes.
func (h *Handle) Wait() <-chan execbox.ExitResult {
	return h.done
}

// Info returns the current session info.
func (h *Handle) Info() execbox.SessionInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	info := execbox.SessionInfo{
		ID:          h.id,
		ContainerID: h.podName,
		Status:      h.status,
		Spec:        h.spec,
		CreatedAt:   h.createdAt,
		ExitCode:    h.exitCode,
		Network:     h.network,
	}

	if h.exitErr != nil {
		info.Error = h.exitErr.Error()
	}

	return info
}

// URL returns the URL to access a container port via port forwarding.
func (h *Handle) URL(port int) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if we already have a port forwarder for this port
	if pf, exists := h.portForwarders[port]; exists {
		return fmt.Sprintf("http://localhost:%d", pf.localPort), nil
	}

	// Start a new port forwarder
	pf, err := h.startPortForward(port)
	if err != nil {
		return "", fmt.Errorf("failed to start port forward: %w", err)
	}

	h.portForwarders[port] = pf
	return fmt.Sprintf("http://localhost:%d", pf.localPort), nil
}

// startPortForward creates a new port forward to the specified remote port.
// Must be called with h.mu locked.
func (h *Handle) startPortForward(remotePort int) (*portForwarder, error) {
	// Create the URL for port forwarding
	req := h.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(h.namespace).
		Name(h.podName).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(h.restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create round tripper: %w", err)
	}

	// Use port 0 for auto-assignment of local port
	ports := []string{fmt.Sprintf("0:%d", remotePort)}

	pf := &portForwarder{
		remotePort: remotePort,
		stopChan:   make(chan struct{}, 1),
		readyChan:  make(chan struct{}, 1),
		errChan:    make(chan error, 1),
	}

	// Create the dialer
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	// Create the port forwarder
	readyWriter := &readyWriter{readyChan: pf.readyChan}
	fw, err := portforward.New(dialer, ports, pf.stopChan, pf.readyChan, readyWriter, readyWriter)
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// Run the port forwarder in a goroutine
	go func() {
		if err := fw.ForwardPorts(); err != nil {
			pf.errChan <- err
		}
	}()

	// Wait for the port forwarder to be ready
	select {
	case <-pf.readyChan:
		// Get the assigned local port
		forwardedPorts, err := fw.GetPorts()
		if err != nil {
			close(pf.stopChan)
			return nil, fmt.Errorf("failed to get forwarded ports: %w", err)
		}
		if len(forwardedPorts) == 0 {
			close(pf.stopChan)
			return nil, fmt.Errorf("no ports were forwarded")
		}
		pf.localPort = int(forwardedPorts[0].Local)

	case err := <-pf.errChan:
		return nil, fmt.Errorf("port forward failed: %w", err)

	case <-time.After(10 * time.Second):
		close(pf.stopChan)
		return nil, fmt.Errorf("timeout waiting for port forward to be ready")
	}

	return pf, nil
}

// SetStatus updates the session status.
func (h *Handle) SetStatus(status execbox.Status) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status = status
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

// SetExitCode updates the exit code.
func (h *Handle) SetExitCode(code int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.exitCode = &code
}

// SetExitError updates the exit error.
func (h *Handle) SetExitError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.exitErr = err
}

// SetStdin sets the stdin writer for the handle.
func (h *Handle) SetStdin(stdin io.WriteCloser) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stdin = stdin
}

// SetAttachStreams connects the pod's stdout/stderr to the handle's BufferedPipes.
// It starts goroutines to copy data from the attach streams to the BufferedPipes.
func (h *Handle) SetAttachStreams(stdout, stderr io.Reader) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Wrap readers as ReadCloser
	if rc, ok := stdout.(io.ReadCloser); ok {
		h.attachStdout = rc
	} else {
		h.attachStdout = io.NopCloser(stdout)
	}

	if rc, ok := stderr.(io.ReadCloser); ok {
		h.attachStderr = rc
	} else {
		h.attachStderr = io.NopCloser(stderr)
	}

	// Start copying from attach streams to BufferedPipes
	go func() {
		_, _ = io.Copy(h.stdout, h.attachStdout)
		h.stdout.Finish()
	}()

	go func() {
		_, _ = io.Copy(h.stderr, h.attachStderr)
		h.stderr.Finish()
	}()
}

// SetNetwork updates the network information.
func (h *Handle) SetNetwork(network *execbox.NetworkInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.network = network
}

// SetCancelWatcher sets the cancel function for the pod watcher.
func (h *Handle) SetCancelWatcher(cancel func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cancelWatcher = cancel
}

// SignalExit signals that the session has exited and sends the result.
func (h *Handle) SignalExit(result execbox.ExitResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update status based on exit result
	if result.Error != nil {
		h.status = execbox.StatusFailed
		h.exitErr = result.Error
	} else if result.Code == 0 {
		h.status = execbox.StatusStopped
	} else {
		h.status = execbox.StatusFailed
	}

	if h.exitCode == nil {
		h.exitCode = &result.Code
	}

	// Send result to done channel (non-blocking)
	select {
	case h.done <- result:
	default:
	}

	// Mark streams as finished
	h.stdout.Finish()
	h.stderr.Finish()

	// Signal exit
	select {
	case <-h.exitSig:
		// Already closed
	default:
		close(h.exitSig)
	}
}

// Close releases all resources.
func (h *Handle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Cancel the pod watcher if it exists
	if h.cancelWatcher != nil {
		h.cancelWatcher()
		h.cancelWatcher = nil
	}

	// Stop all port forwarders
	for _, pf := range h.portForwarders {
		close(pf.stopChan)
	}
	h.portForwarders = make(map[int]*portForwarder)

	// Close stdin if it exists
	if h.stdin != nil {
		h.stdin.Close()
	}

	// Close attach streams
	if h.attachStdout != nil {
		h.attachStdout.Close()
	}
	if h.attachStderr != nil {
		h.attachStderr.Close()
	}

	// Close stdout and stderr
	h.stdout.Close()
	h.stderr.Close()

	return nil
}

// readyWriter implements io.Writer for port forward ready/error channels.
type readyWriter struct {
	readyChan chan struct{}
}

func (w *readyWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// noopWriteCloser is a no-op io.WriteCloser used when stdin is not available.
type noopWriteCloser struct{}

func (n *noopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n *noopWriteCloser) Close() error {
	return nil
}
