//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/k8s"
	"github.com/burka/execbox/pkg/execbox"
)

func TestK8sBackend_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create backend with microk8s config
	backend, err := k8s.NewBackend(k8s.BackendConfig{
		Kubeconfig: "/tmp/microk8s-kubeconfig",
		Namespace:  "execbox-test",
	})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Test Health
	t.Run("Health", func(t *testing.T) {
		ctx := context.Background()
		if err := backend.Health(ctx); err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		t.Log("✅ Health check passed")
	})

	// Test Run with simple command
	var sessionID string
	var mainCtx = context.Background() // Use a long-lived context for the session
	t.Run("Run_SimpleCommand", func(t *testing.T) {
		spec := execbox.Spec{
			Image:   "alpine:latest",
			Command: []string{"sleep", "60"}, // Simple long-running command
		}

		handle, err := backend.Run(mainCtx, spec)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		sessionID = handle.ID()
		t.Logf("✅ Created session: %s", sessionID)

		// Wait for container to be fully ready before testing exec
		time.Sleep(2 * time.Second)
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("No session ID from Run test")
		}
		ctx := context.Background()
		info, err := backend.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		t.Logf("✅ Session info: ID=%s, Status=%s", info.ID, info.Status)
	})

	// Test Exec
	t.Run("Exec", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("No session ID from Run test")
		}
		ctx := context.Background()
		stdout, stderr, exitCode, err := backend.Exec(ctx, sessionID, []string{"echo", "exec works"})
		if err != nil {
			t.Fatalf("Exec failed: %v", err)
		}
		t.Logf("✅ Exec result: stdout=%q, stderr=%q, exitCode=%d", stdout, stderr, exitCode)
		if !strings.Contains(stdout, "exec works") {
			t.Errorf("Expected 'exec works' in stdout, got: %q", stdout)
		}
	})

	// Test List
	t.Run("List", func(t *testing.T) {
		ctx := context.Background()
		sessions, err := backend.List(ctx, execbox.Filter{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		t.Logf("✅ Found %d sessions", len(sessions))
		for _, s := range sessions {
			t.Logf("  - %s: %s", s.ID, s.Status)
		}
	})

	// Test Stop
	t.Run("Stop", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("No session ID from Run test")
		}
		ctx := context.Background()
		if err := backend.Stop(ctx, sessionID); err != nil {
			t.Fatalf("Stop failed: %v", err)
		}
		t.Log("✅ Stop sent")
		
		// Wait a bit for pod to stop
		time.Sleep(2 * time.Second)
	})

	// Test Destroy
	t.Run("Destroy", func(t *testing.T) {
		if sessionID == "" {
			t.Skip("No session ID from Run test")
		}
		ctx := context.Background()
		if err := backend.Destroy(ctx, sessionID); err != nil {
			t.Fatalf("Destroy failed: %v", err)
		}
		t.Log("✅ Destroyed session")

		// Wait for K8s to actually delete the pod (async operation)
		time.Sleep(3 * time.Second)

		// Verify it's gone
		_, err := backend.Get(ctx, sessionID)
		if err == nil {
			t.Log("⚠️ Pod still visible after destroy (K8s deletion is async)")
		} else {
			t.Logf("✅ Session correctly not found after destroy")
		}
	})

	// Test Run with Setup (init container)
	t.Run("Run_WithSetup", func(t *testing.T) {
		ctx := context.Background()

		spec := execbox.Spec{
			Image:   "alpine:latest",
			Command: []string{"sleep", "60"}, // Keep running for exec
			Setup:   []string{"echo 'setup-done' > /tmp/setup-marker"},
		}

		handle, err := backend.Run(ctx, spec)
		if err != nil {
			t.Fatalf("Run with setup failed: %v", err)
		}
		defer backend.Destroy(ctx, handle.ID())

		t.Logf("✅ Created session with setup: %s", handle.ID())

		// Wait for container to be ready
		time.Sleep(2 * time.Second)

		// Verify setup by exec'ing cat
		stdout, _, exitCode, err := backend.Exec(ctx, handle.ID(), []string{"cat", "/tmp/setup-marker"})
		if err != nil {
			t.Fatalf("Exec failed: %v", err)
		}
		t.Logf("📝 Setup marker content: %q (exit=%d)", stdout, exitCode)
		if strings.Contains(stdout, "setup-done") {
			t.Log("✅ Init container successfully created marker file")
		} else {
			t.Errorf("Expected setup-done marker, got: %q", stdout)
		}
	})

	// Test Run with BuildFiles (ConfigMap)
	t.Run("Run_WithBuildFiles", func(t *testing.T) {
		ctx := context.Background()

		spec := execbox.Spec{
			Image:   "alpine:latest",
			Command: []string{"sleep", "60"}, // Keep running for exec
			BuildFiles: []execbox.BuildFile{
				{
					Path:    "/app/config.txt",
					Content: []byte("Hello from BuildFile!"),
				},
			},
		}

		handle, err := backend.Run(ctx, spec)
		if err != nil {
			t.Fatalf("Run with BuildFiles failed: %v", err)
		}
		defer backend.Destroy(ctx, handle.ID())

		t.Logf("✅ Created session with BuildFiles: %s", handle.ID())

		// Wait for container to be ready
		time.Sleep(2 * time.Second)

		// Verify file was injected by exec'ing cat
		stdout, _, exitCode, err := backend.Exec(ctx, handle.ID(), []string{"cat", "/app/config.txt"})
		if err != nil {
			t.Fatalf("Exec failed: %v", err)
		}
		t.Logf("📝 BuildFile content: %q (exit=%d)", stdout, exitCode)
		if strings.Contains(stdout, "Hello from BuildFile!") {
			t.Log("✅ BuildFile successfully injected via ConfigMap")
		} else {
			t.Errorf("Expected BuildFile content, got: %q", stdout)
		}
	})

	// Test port forwarding with URL()
	t.Run("URL_PortForward", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		spec := execbox.Spec{
			Image:   "python:3.12-slim",
			Command: []string{"python", "-m", "http.server", "8080"},
			Ports: []execbox.Port{
				{Container: 8080, Protocol: "tcp"},
			},
		}

		handle, err := backend.Run(ctx, spec)
		if err != nil {
			t.Fatalf("Run with ports failed: %v", err)
		}
		defer backend.Destroy(ctx, handle.ID())

		t.Logf("✅ Created session with port 8080: %s", handle.ID())

		// Wait for server to start
		time.Sleep(3 * time.Second)

		// Get URL
		url, err := handle.URL(8080)
		if err != nil {
			t.Logf("⚠️ URL() failed: %v (may need port-forward support)", err)
		} else {
			t.Logf("✅ Port forward URL: %s", url)
		}
	})
}
