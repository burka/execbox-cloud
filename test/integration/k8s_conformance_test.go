//go:build integration

// Package integration tests the K8s backend against execbox conformance scenarios.
// These tests mirror the execbox/test/backend tests to verify K8s backend compatibility.
package integration

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/k8s"
	"github.com/burka/execbox/pkg/execbox"
	"github.com/burka/execbox/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createK8sExecbox creates an Execbox instance backed by K8s.
func createK8sExecbox(t *testing.T) execbox.Execbox {
	t.Helper()

	backend, err := k8s.NewBackend(k8s.BackendConfig{
		Kubeconfig: "/tmp/microk8s-kubeconfig",
		Namespace:  "execbox-conformance",
	})
	require.NoError(t, err, "Failed to create K8s backend")

	// Create session manager
	manager := session.NewManager(backend, 1024*1024) // 1MB buffer

	// Return a wrapper that implements Execbox
	return &k8sExecbox{
		backend: backend,
		manager: manager,
	}
}

// k8sExecbox wraps the K8s backend to implement execbox.Execbox interface.
type k8sExecbox struct {
	backend *k8s.Backend
	manager *session.Manager
}

func (e *k8sExecbox) Run(ctx context.Context, spec execbox.Spec) (execbox.Session, error) {
	return e.manager.Run(ctx, spec)
}

func (e *k8sExecbox) RunWithSpec(ctx context.Context, minimal execbox.MinimalSpec, image string, command []string) (execbox.Session, error) {
	spec := execbox.Spec{
		Image:   image,
		Command: command,
	}
	return e.Run(ctx, spec)
}

func (e *k8sExecbox) Attach(ctx context.Context, id string) (execbox.Session, error) {
	return e.manager.Attach(ctx, id)
}

func (e *k8sExecbox) Get(ctx context.Context, id string) (execbox.SessionInfo, error) {
	return e.manager.Get(ctx, id)
}

func (e *k8sExecbox) List(ctx context.Context, filter execbox.Filter) ([]execbox.SessionInfo, error) {
	return e.manager.List(ctx, filter)
}

func (e *k8sExecbox) Stop(ctx context.Context, id string) error {
	return e.manager.Stop(ctx, id)
}

func (e *k8sExecbox) Kill(ctx context.Context, id string) error {
	return e.manager.Kill(ctx, id)
}

func (e *k8sExecbox) Destroy(ctx context.Context, id string) error {
	return e.manager.Destroy(ctx, id)
}

func (e *k8sExecbox) Exec(ctx context.Context, sessionID string, cmd []string) (string, string, int, error) {
	return e.manager.Exec(ctx, sessionID, cmd)
}

func (e *k8sExecbox) Health(ctx context.Context) error {
	return e.backend.Health(ctx)
}

func (e *k8sExecbox) Close() error {
	e.manager.Close()
	return e.backend.Close()
}

// shellCmd returns a shell command for alpine
func shellCmd(script string) []string {
	return []string{"sh", "-c", script}
}

// ============================================================================
// BASIC EXECUTION TESTS (mirrors execbox/test/backend/backend_test.go)
// ============================================================================

func TestK8sConformance_Echo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("echo hello world"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello world")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Echo test passed: %q", string(out))
}

func TestK8sConformance_ExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("exit 42"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	result := <-sess.Wait()
	assert.Equal(t, 42, result.Code)
	t.Logf("✅ Exit code test passed: %d", result.Code)
}

func TestK8sConformance_Stdin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("cat"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	_, err = sess.Stdin().Write([]byte("hello from stdin"))
	require.NoError(t, err)
	sess.Stdin().Close()

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello from stdin")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Stdin test passed: %q", string(out))
}

func TestK8sConformance_Stderr(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("echo error message >&2"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Note: K8s pod logs don't separate stdout/stderr - all output goes to stdout.
	// For completed pods, stderr will be empty and all output appears in stdout.
	stdout, _ := io.ReadAll(sess.Stdout())
	errOut, err := io.ReadAll(sess.Stderr())
	require.NoError(t, err)

	// The error message should appear in EITHER stdout or stderr
	combined := string(stdout) + string(errOut)
	assert.Contains(t, combined, "error message", "K8s logs combine stdout/stderr")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Stderr test passed: stdout=%q, stderr=%q", string(stdout), string(errOut))
}

func TestK8sConformance_MixedOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("echo stdout; echo stderr >&2; echo stdout2"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	var stdout, stderr string
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		out, _ := io.ReadAll(sess.Stdout())
		stdout = string(out)
	}()

	go func() {
		defer wg.Done()
		out, _ := io.ReadAll(sess.Stderr())
		stderr = string(out)
	}()

	wg.Wait()

	// Note: K8s pod logs don't separate stdout/stderr - all output goes to stdout.
	// For completed pods, all output appears in stdout and stderr is empty.
	combined := stdout + stderr
	assert.Contains(t, combined, "stdout")
	assert.Contains(t, combined, "stdout2")
	assert.Contains(t, combined, "stderr", "K8s logs combine stdout/stderr")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Mixed output test passed: stdout=%q, stderr=%q (K8s combines streams)", stdout, stderr)
}

func TestK8sConformance_EnvVar(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("echo $MY_VAR"),
		Env:     map[string]string{"MY_VAR": "test_value"},
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.Contains(t, string(out), "test_value")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Env var test passed: %q", string(out))
}

func TestK8sConformance_WorkDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("pwd"),
		WorkDir: "/tmp",
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.Contains(t, string(out), "/tmp")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ WorkDir test passed: %q", string(out))
}

func TestK8sConformance_Kill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("sleep 300"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Give pod time to start
	time.Sleep(2 * time.Second)

	// Kill the session
	err = exec.Kill(context.Background(), sess.ID())
	require.NoError(t, err)

	// Wait for exit
	select {
	case result := <-sess.Wait():
		t.Logf("✅ Kill test passed: exit code %d", result.Code)
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for kill")
	}
}

func TestK8sConformance_Stop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	// Use a simple sleep command that responds to SIGTERM
	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: []string{"sleep", "300"},
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Give pod time to start
	time.Sleep(2 * time.Second)

	// Stop the session (graceful SIGTERM)
	err = exec.Stop(context.Background(), sess.ID())
	require.NoError(t, err)

	// Wait for exit - sleep responds to SIGTERM by exiting
	select {
	case result := <-sess.Wait():
		// sleep exits with code 143 (128 + 15 = SIGTERM) when killed by signal
		t.Logf("✅ Stop test passed: exit code %d", result.Code)
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for stop")
	}
}

func TestK8sConformance_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("sleep 30"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Give pod time to start
	time.Sleep(2 * time.Second)

	info, err := exec.Get(context.Background(), sess.ID())
	require.NoError(t, err)
	assert.Equal(t, sess.ID(), info.ID)
	assert.Equal(t, execbox.StatusRunning, info.Status)
	t.Logf("✅ Get test passed: ID=%s, Status=%s", info.ID, info.Status)
}

func TestK8sConformance_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("sleep 30"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Give pod time to start
	time.Sleep(2 * time.Second)

	sessions, err := exec.List(context.Background(), execbox.Filter{})
	require.NoError(t, err)

	found := false
	for _, s := range sessions {
		if s.ID == sess.ID() {
			found = true
			break
		}
	}
	assert.True(t, found, "Session should be in list")
	t.Logf("✅ List test passed: found %d sessions", len(sessions))
}

func TestK8sConformance_Exec(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("sleep 60"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Give pod time to start
	time.Sleep(2 * time.Second)

	stdout, stderr, exitCode, err := exec.Exec(context.Background(), sess.ID(), []string{"echo", "exec works"})
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "exec works")
	assert.Empty(t, stderr)
	t.Logf("✅ Exec test passed: stdout=%q, exitCode=%d", stdout, exitCode)
}

func TestK8sConformance_LargeOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	// Generate 100KB of output
	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("dd if=/dev/zero bs=1024 count=100 | tr '\\0' 'x'"),
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(out), 100*1024, "Should have at least 100KB output")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Large output test passed: %d bytes", len(out))
}

func TestK8sConformance_ConcurrentSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	const numSessions = 3
	var wg sync.WaitGroup
	results := make(chan string, numSessions)
	errors := make(chan error, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			sess, err := exec.Run(context.Background(), execbox.Spec{
				Image:   "alpine:latest",
				Command: shellCmd("echo session-" + string(rune('0'+id))),
			})
			if err != nil {
				errors <- err
				return
			}
			defer exec.Destroy(context.Background(), sess.ID())

			out, err := io.ReadAll(sess.Stdout())
			if err != nil {
				errors <- err
				return
			}

			<-sess.Wait()
			results <- strings.TrimSpace(string(out))
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent session error: %v", err)
	}

	count := 0
	for result := range results {
		count++
		t.Logf("Concurrent result: %s", result)
	}

	assert.Equal(t, numSessions, count, "All sessions should complete")
	t.Logf("✅ Concurrent sessions test passed: %d sessions", count)
}

// ============================================================================
// K8s-SPECIFIC FEATURE TESTS
// ============================================================================

func TestK8sConformance_SetupCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("cat /tmp/setup-marker"),
		Setup:   []string{"echo 'setup-done' > /tmp/setup-marker"},
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.Contains(t, string(out), "setup-done")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ Setup commands test passed: %q", string(out))
}

func TestK8sConformance_BuildFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "alpine:latest",
		Command: shellCmd("cat /app/config.txt"),
		BuildFiles: []execbox.BuildFile{
			{
				Path:    "/app/config.txt",
				Content: []byte("Hello from BuildFile!"),
			},
		},
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	out, err := io.ReadAll(sess.Stdout())
	require.NoError(t, err)
	assert.Contains(t, string(out), "Hello from BuildFile!")

	result := <-sess.Wait()
	assert.Equal(t, 0, result.Code)
	t.Logf("✅ BuildFiles test passed: %q", string(out))
}

func TestK8sConformance_PortForward(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conformance test in short mode")
	}

	exec := createK8sExecbox(t)
	defer exec.Close()

	sess, err := exec.Run(context.Background(), execbox.Spec{
		Image:   "python:3.12-slim",
		Command: []string{"python", "-m", "http.server", "8080"},
		Ports: []execbox.Port{
			{Container: 8080, Protocol: "tcp"},
		},
	})
	require.NoError(t, err)
	defer exec.Destroy(context.Background(), sess.ID())

	// Wait for server to start
	time.Sleep(3 * time.Second)

	url, err := sess.URL(8080)
	if err != nil {
		t.Logf("⚠️ Port forwarding not available: %v", err)
		t.Skip("Port forwarding requires kubectl")
	}

	assert.Contains(t, url, "localhost")
	t.Logf("✅ Port forward test passed: %s", url)
}
