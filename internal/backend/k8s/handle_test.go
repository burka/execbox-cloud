package k8s

import (
	"testing"

	"github.com/burka/execbox/pkg/execbox"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestHandleImplementsInterface verifies Handle implements execbox.Handle interface.
func TestHandleImplementsInterface(t *testing.T) {
	// Create a minimal handle
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		&kubernetes.Clientset{},
		&rest.Config{},
	)

	// Verify interface implementation at compile time
	var _ execbox.Handle = h
	var _ execbox.HandleIdentity = h
	var _ execbox.HandleIO = h
	var _ execbox.HandleWaiter = h
	var _ execbox.HandleNetwork = h
	var _ execbox.HandleMetadata = h

	// Basic functionality test
	if h.ID() != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", h.ID())
	}

	info := h.Info()
	if info.ID != "test-id" {
		t.Errorf("expected Info.ID 'test-id', got '%s'", info.ID)
	}
	if info.ContainerID != "test-pod" {
		t.Errorf("expected Info.ContainerID 'test-pod', got '%s'", info.ContainerID)
	}
	if info.Status != execbox.StatusRunning {
		t.Errorf("expected Status Running, got %s", info.Status)
	}
}

func TestHandleIO(t *testing.T) {
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		&kubernetes.Clientset{},
		&rest.Config{},
	)

	// Test that I/O streams are not nil
	if h.Stdin() == nil {
		t.Error("Stdin() returned nil")
	}
	if h.Stdout() == nil {
		t.Error("Stdout() returned nil")
	}
	if h.Stderr() == nil {
		t.Error("Stderr() returned nil")
	}

	// Test Wait channel
	waitChan := h.Wait()
	if waitChan == nil {
		t.Error("Wait() returned nil channel")
	}
}

func TestHandleClose(t *testing.T) {
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		&kubernetes.Clientset{},
		&rest.Config{},
	)

	err := h.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestHandleStatusUpdates(t *testing.T) {
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		&kubernetes.Clientset{},
		&rest.Config{},
	)

	// Test initial status
	if h.Info().Status != execbox.StatusRunning {
		t.Errorf("expected initial status Running, got %s", h.Info().Status)
	}

	// Test SetStopping
	h.SetStopping()
	if h.Info().Status != execbox.StatusStopping {
		t.Errorf("expected status Stopping after SetStopping, got %s", h.Info().Status)
	}

	// Test SetKilled
	h.SetKilled()
	if h.Info().Status != execbox.StatusKilled {
		t.Errorf("expected status Killed after SetKilled, got %s", h.Info().Status)
	}
}

func TestHandleExitCode(t *testing.T) {
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		&kubernetes.Clientset{},
		&rest.Config{},
	)

	// Initially no exit code
	info := h.Info()
	if info.ExitCode != nil {
		t.Errorf("expected nil exit code initially, got %v", *info.ExitCode)
	}

	// Set exit code
	h.SetExitCode(42)
	info = h.Info()
	if info.ExitCode == nil {
		t.Error("expected non-nil exit code after SetExitCode")
	} else if *info.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", *info.ExitCode)
	}
}

func TestHandleSignalExit(t *testing.T) {
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		&kubernetes.Clientset{},
		&rest.Config{},
	)

	// Signal exit with code 0
	result := execbox.ExitResult{
		Code:  0,
		Error: nil,
	}
	h.SignalExit(result)

	// Check that status is updated
	info := h.Info()
	if info.Status != execbox.StatusStopped {
		t.Errorf("expected status Stopped after successful exit, got %s", info.Status)
	}
	if info.ExitCode == nil || *info.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %v", info.ExitCode)
	}

	// Check that result is sent to Wait channel
	select {
	case r := <-h.Wait():
		if r.Code != 0 {
			t.Errorf("expected exit code 0 from Wait channel, got %d", r.Code)
		}
	default:
		t.Error("expected result in Wait channel, but channel is empty")
	}
}
