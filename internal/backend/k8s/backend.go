package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/burka/execbox/pkg/execbox"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// BackendConfig holds configuration for the Kubernetes backend.
type BackendConfig struct {
	Kubeconfig       string             // Path to kubeconfig (empty = in-cluster)
	Namespace        string             // Default namespace (default: "execbox")
	ServiceAccount   string             // Service account for pods
	ImagePullSecrets []string           // Image pull secret names
	StorageClass     string             // Storage class for PVCs
	DefaultResources *execbox.Resources // Default resource limits
	Labels           map[string]string  // Labels to add to all resources
}

// Backend implements execbox.Backend for Kubernetes.
type Backend struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
	config     BackendConfig
	handles    map[string]*Handle
	mu         sync.RWMutex
}

// NewBackend creates a new Kubernetes backend.
func NewBackend(cfg BackendConfig) (*Backend, error) {
	// Set defaults
	if cfg.Namespace == "" {
		cfg.Namespace = "execbox"
	}
	if cfg.Labels == nil {
		cfg.Labels = make(map[string]string)
	}

	// Try to load kubeconfig
	var restConfig *rest.Config
	var err error

	if cfg.Kubeconfig == "" {
		// Try in-cluster config first
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			// Fall back to default kubeconfig location
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}
			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
			restConfig, err = kubeConfig.ClientConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
			}
		}
	} else {
		// Use specified kubeconfig
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", cfg.Kubeconfig, err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	backend := &Backend{
		clientset:  clientset,
		restConfig: restConfig,
		config:     cfg,
		handles:    make(map[string]*Handle),
	}

	// Ensure namespace exists
	if err := backend.ensureNamespace(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	return backend, nil
}

// ensureNamespace creates the namespace if it doesn't exist.
func (b *Backend) ensureNamespace(ctx context.Context) error {
	_, err := b.clientset.CoreV1().Namespaces().Get(ctx, b.config.Namespace, metav1.GetOptions{})
	if err == nil {
		return nil // Already exists
	}

	// Create namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.config.Namespace,
			Labels: map[string]string{
				"execbox.io/managed-by": "execbox",
			},
		},
	}

	_, err = b.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return nil
}

// Name returns "kubernetes".
func (b *Backend) Name() string {
	return "kubernetes"
}

// Run creates a Kubernetes pod from spec and returns a Handle.
func (b *Backend) Run(ctx context.Context, spec execbox.Spec) (execbox.Handle, error) {
	// Generate unique session ID
	sessionID := uuid.New().String()

	// Create ConfigMap for build files if present
	if len(spec.BuildFiles) > 0 {
		cm := BuildFilesToConfigMap(spec.BuildFiles, sessionID, b.config.Namespace)
		if _, err := b.clientset.CoreV1().ConfigMaps(b.config.Namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create configmap: %w", err)
		}
	}

	// Convert spec to pod
	pod := SpecToPod(spec, sessionID, b.config.Namespace, b.config.Labels)

	// Create pod
	createdPod, err := b.clientset.CoreV1().Pods(b.config.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		// Cleanup configmap on failure
		if len(spec.BuildFiles) > 0 {
			cmName := fmt.Sprintf("execbox-files-%s", sessionID[:8])
			_ = b.clientset.CoreV1().ConfigMaps(b.config.Namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
		}
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	// Wait for pod to be running
	if err := b.waitForPodRunning(ctx, createdPod.Name, 60*time.Second); err != nil {
		// Cleanup on failure
		_ = b.destroyResources(ctx, sessionID)
		return nil, fmt.Errorf("pod failed to start: %w", err)
	}

	// Create handle
	handle := NewHandle(sessionID, createdPod.Name, b.config.Namespace, spec, b.clientset, b.restConfig)

	// Register handle
	b.mu.Lock()
	b.handles[sessionID] = handle
	b.mu.Unlock()

	return handle, nil
}

// waitForPodRunning waits for a pod to reach Running state.
func (b *Backend) waitForPodRunning(ctx context.Context, podName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod to run: %w", ctx.Err())
		case <-ticker.C:
			pod, err := b.clientset.CoreV1().Pods(b.config.Namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod: %w", err)
			}

			switch pod.Status.Phase {
			case corev1.PodRunning:
				return nil
			case corev1.PodFailed, corev1.PodSucceeded:
				return fmt.Errorf("pod entered terminal state: %s", pod.Status.Phase)
			}
		}
	}
}

// Attach reconnects to an existing pod.
func (b *Backend) Attach(ctx context.Context, id string) (execbox.Handle, error) {
	// Check local handles first
	b.mu.RLock()
	handle, ok := b.handles[id]
	b.mu.RUnlock()

	if ok && handle != nil {
		return handle, nil
	}

	// Try to find pod by label selector
	labelSelector := fmt.Sprintf("execbox.io/session-id=%s", id)
	pods, err := b.clientset.CoreV1().Pods(b.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, execbox.ErrSessionNotFound
	}

	pod := &pods.Items[0]

	// Recreate handle with minimal spec (we lost the original)
	// Combine Command and Args from Kubernetes into execbox Command
	cmd := append([]string{}, pod.Spec.Containers[0].Command...)
	cmd = append(cmd, pod.Spec.Containers[0].Args...)

	spec := execbox.Spec{
		Image:   pod.Spec.Containers[0].Image,
		Command: cmd,
	}

	handle = NewHandle(id, pod.Name, b.config.Namespace, spec, b.clientset, b.restConfig)

	// Register handle
	b.mu.Lock()
	b.handles[id] = handle
	b.mu.Unlock()

	return handle, nil
}

// Get returns session info for a given session ID.
func (b *Backend) Get(ctx context.Context, id string) (execbox.SessionInfo, error) {
	// Check local handles first
	b.mu.RLock()
	handle, ok := b.handles[id]
	b.mu.RUnlock()

	if ok && handle != nil {
		return handle.Info(), nil
	}

	// Try to find pod by label selector
	labelSelector := fmt.Sprintf("execbox.io/session-id=%s", id)
	pods, err := b.clientset.CoreV1().Pods(b.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return execbox.SessionInfo{}, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return execbox.SessionInfo{}, execbox.ErrSessionNotFound
	}

	pod := &pods.Items[0]
	return PodToSessionInfo(pod, id), nil
}

// List returns all sessions matching the filter.
func (b *Backend) List(ctx context.Context, filter execbox.Filter) ([]execbox.SessionInfo, error) {
	// Build label selector
	labelSelector := "execbox.io/managed-by=execbox"

	// Add custom label filters
	if len(filter.Labels) > 0 {
		for k, v := range filter.Labels {
			labelSelector = fmt.Sprintf("%s,%s=%s", labelSelector, k, v)
		}
	}

	pods, err := b.clientset.CoreV1().Pods(b.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var results []execbox.SessionInfo
	for i := range pods.Items {
		pod := &pods.Items[i]

		sessionID := pod.Labels["execbox.io/session-id"]
		if sessionID == "" {
			continue
		}

		info := PodToSessionInfo(pod, sessionID)

		// Apply status filter
		if filter.Status != nil && info.Status != *filter.Status {
			continue
		}

		results = append(results, info)
	}

	return results, nil
}

// Stop gracefully stops a pod by sending SIGTERM.
func (b *Backend) Stop(ctx context.Context, id string) error {
	b.mu.RLock()
	handle := b.handles[id]
	b.mu.RUnlock()

	if handle == nil {
		// Try to find pod
		labelSelector := fmt.Sprintf("execbox.io/session-id=%s", id)
		pods, err := b.clientset.CoreV1().Pods(b.config.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}
		if len(pods.Items) == 0 {
			return execbox.ErrSessionNotFound
		}
	}

	// Send SIGTERM to process 1
	_, _, _, err := b.Exec(ctx, id, []string{"kill", "-TERM", "1"})
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	if handle != nil {
		handle.SetStopping()
	}

	return nil
}

// Kill forcefully terminates a pod.
func (b *Backend) Kill(ctx context.Context, id string) error {
	b.mu.RLock()
	handle := b.handles[id]
	b.mu.RUnlock()

	// Find pod
	labelSelector := fmt.Sprintf("execbox.io/session-id=%s", id)
	pods, err := b.clientset.CoreV1().Pods(b.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return execbox.ErrSessionNotFound
	}

	pod := &pods.Items[0]

	// Delete pod with zero grace period
	gracePeriod := int64(0)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	err = b.clientset.CoreV1().Pods(b.config.Namespace).Delete(ctx, pod.Name, deleteOptions)
	if err != nil {
		return fmt.Errorf("failed to kill pod: %w", err)
	}

	if handle != nil {
		handle.SetKilled()
	}

	return nil
}

// Destroy permanently removes a session and all its resources.
func (b *Backend) Destroy(ctx context.Context, id string) error {
	b.mu.Lock()
	handle := b.handles[id]
	delete(b.handles, id)
	b.mu.Unlock()

	if handle != nil {
		handle.Close()
	}

	return b.destroyResources(ctx, id)
}

// destroyResources removes all resources associated with a session.
func (b *Backend) destroyResources(ctx context.Context, sessionID string) error {
	labelSelector := fmt.Sprintf("execbox.io/session-id=%s", sessionID)

	// Delete pods
	if err := b.clientset.CoreV1().Pods(b.config.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	}); err != nil {
		return fmt.Errorf("failed to delete pods: %w", err)
	}

	// Delete ConfigMaps
	if err := b.clientset.CoreV1().ConfigMaps(b.config.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	}); err != nil {
		return fmt.Errorf("failed to delete configmaps: %w", err)
	}

	// Delete PVCs
	if err := b.clientset.CoreV1().PersistentVolumeClaims(b.config.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	}); err != nil {
		return fmt.Errorf("failed to delete pvcs: %w", err)
	}

	return nil
}

// Exec runs a command in a running pod.
func (b *Backend) Exec(ctx context.Context, sessionID string, cmd []string) (stdout, stderr string, exitCode int, err error) {
	// Find pod
	labelSelector := fmt.Sprintf("execbox.io/session-id=%s", sessionID)
	pods, err := b.clientset.CoreV1().Pods(b.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", "", -1, execbox.ErrSessionNotFound
	}

	pod := &pods.Items[0]

	// Execute command using SPDY executor
	req := b.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(b.config.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command:   cmd,
			Container: pod.Spec.Containers[0].Name,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(b.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create executor: %w", err)
	}

	// Create buffers for output
	var stdoutBuf, stderrBuf streamBuffer

	// Execute
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	})

	// Determine exit code
	exitCode = 0
	if err != nil {
		// Try to extract exit code from error
		if exitErr, ok := err.(ExitCoder); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// ExitCoder is an interface for errors that include an exit code.
type ExitCoder interface {
	ExitCode() int
}

// streamBuffer is a simple buffer that implements io.Writer for capturing output.
type streamBuffer struct {
	data []byte
	mu   sync.Mutex
}

func (sb *streamBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.data = append(sb.data, p...)
	return len(p), nil
}

func (sb *streamBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return string(sb.data)
}

// Health checks if the Kubernetes API is reachable.
func (b *Backend) Health(ctx context.Context) error {
	_, err := b.clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("kubernetes API unhealthy: %w", err)
	}
	return nil
}

// Close releases all resources.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, handle := range b.handles {
		if handle != nil {
			handle.Close()
		}
	}

	b.handles = make(map[string]*Handle)

	return nil
}
