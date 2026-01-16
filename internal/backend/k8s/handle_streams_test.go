package k8s

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/burka/execbox/pkg/execbox"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// TestSetCompletedStreams_DataIsReadable verifies that data written via SetCompletedStreams
// is actually readable from the handle's Stdout/Stderr.
func TestSetCompletedStreams_DataIsReadable(t *testing.T) {
	tests := []struct {
		name           string
		stdout         string
		stderr         string
		exitCode       int
		wantStatus     execbox.Status
		wantStdout     string
		wantStderr     string
		wantExitCode   int
	}{
		{
			name:         "successful exit with output",
			stdout:       "hello world\nline 2\n",
			stderr:       "",
			exitCode:     0,
			wantStatus:   execbox.StatusStopped,
			wantStdout:   "hello world\nline 2\n",
			wantStderr:   "",
			wantExitCode: 0,
		},
		{
			name:         "failed exit with stderr",
			stdout:       "partial output\n",
			stderr:       "error: something failed\n",
			exitCode:     1,
			wantStatus:   execbox.StatusFailed,
			wantStdout:   "partial output\n",
			wantStderr:   "error: something failed\n",
			wantExitCode: 1,
		},
		{
			name:         "empty output success",
			stdout:       "",
			stderr:       "",
			exitCode:     0,
			wantStatus:   execbox.StatusStopped,
			wantStdout:   "",
			wantStderr:   "",
			wantExitCode: 0,
		},
		{
			name:         "large output",
			stdout:       strings.Repeat("x", 10000),
			stderr:       strings.Repeat("e", 5000),
			exitCode:     0,
			wantStatus:   execbox.StatusStopped,
			wantStdout:   strings.Repeat("x", 10000),
			wantStderr:   strings.Repeat("e", 5000),
			wantExitCode: 0,
		},
		{
			name:         "binary-like data",
			stdout:       "binary\x00data\x01here",
			stderr:       "",
			exitCode:     0,
			wantStatus:   execbox.StatusStopped,
			wantStdout:   "binary\x00data\x01here",
			wantStderr:   "",
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
				fake.NewSimpleClientset(), &rest.Config{})

			// Act
			h.SetCompletedStreams(tt.stdout, tt.stderr, tt.exitCode)

			// Assert - verify status and exit code
			info := h.Info()
			if info.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", info.Status, tt.wantStatus)
			}
			if info.ExitCode == nil {
				t.Fatal("ExitCode is nil")
			}
			if *info.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", *info.ExitCode, tt.wantExitCode)
			}

			// Assert - verify stdout is readable and contains expected data
			gotStdout, err := io.ReadAll(h.Stdout())
			if err != nil {
				t.Fatalf("Failed to read stdout: %v", err)
			}
			if string(gotStdout) != tt.wantStdout {
				t.Errorf("Stdout = %q (len=%d), want %q (len=%d)",
					truncate(string(gotStdout), 50), len(gotStdout),
					truncate(tt.wantStdout, 50), len(tt.wantStdout))
			}

			// Assert - verify stderr is readable and contains expected data
			gotStderr, err := io.ReadAll(h.Stderr())
			if err != nil {
				t.Fatalf("Failed to read stderr: %v", err)
			}
			if string(gotStderr) != tt.wantStderr {
				t.Errorf("Stderr = %q (len=%d), want %q (len=%d)",
					truncate(string(gotStderr), 50), len(gotStderr),
					truncate(tt.wantStderr, 50), len(tt.wantStderr))
			}

			// Assert - verify Wait channel receives exit result
			select {
			case result := <-h.Wait():
				if result.Code != tt.wantExitCode {
					t.Errorf("Wait result Code = %d, want %d", result.Code, tt.wantExitCode)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Wait channel did not receive result")
			}
		})
	}
}

// TestSetAttachStreams_NilStderr verifies that nil stderr is handled correctly.
// This is the behavior used when K8s logs API combines stdout/stderr.
func TestSetAttachStreams_NilStderr(t *testing.T) {
	h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
		fake.NewSimpleClientset(), &rest.Config{})

	// Create a reader that simulates combined logs
	combinedLogs := "stdout line 1\nstderr line 1\nstdout line 2\n"
	reader := strings.NewReader(combinedLogs)

	// Act - set streams with nil stderr (K8s logs API behavior)
	h.SetAttachStreams(reader, nil)

	// Assert - stdout should be readable
	gotStdout, err := io.ReadAll(h.Stdout())
	if err != nil {
		t.Fatalf("Failed to read stdout: %v", err)
	}
	if string(gotStdout) != combinedLogs {
		t.Errorf("Stdout = %q, want %q", gotStdout, combinedLogs)
	}

	// Assert - stderr should return EOF immediately (no data)
	gotStderr, err := io.ReadAll(h.Stderr())
	if err != nil {
		t.Fatalf("Failed to read stderr: %v", err)
	}
	if len(gotStderr) != 0 {
		t.Errorf("Stderr = %q, want empty", gotStderr)
	}
}

// TestSetAttachStreams_NilStdout verifies that nil stdout is handled correctly.
func TestSetAttachStreams_NilStdout(t *testing.T) {
	h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
		fake.NewSimpleClientset(), &rest.Config{})

	// Create stderr only
	stderrData := "error output\n"
	reader := strings.NewReader(stderrData)

	// Act - set streams with nil stdout
	h.SetAttachStreams(nil, reader)

	// Assert - stdout should return EOF immediately
	gotStdout, err := io.ReadAll(h.Stdout())
	if err != nil {
		t.Fatalf("Failed to read stdout: %v", err)
	}
	if len(gotStdout) != 0 {
		t.Errorf("Stdout = %q, want empty", gotStdout)
	}

	// Assert - stderr should be readable
	gotStderr, err := io.ReadAll(h.Stderr())
	if err != nil {
		t.Fatalf("Failed to read stderr: %v", err)
	}
	if string(gotStderr) != stderrData {
		t.Errorf("Stderr = %q, want %q", gotStderr, stderrData)
	}
}

// TestSetAttachStreams_BothNil verifies that both nil streams are handled.
func TestSetAttachStreams_BothNil(t *testing.T) {
	h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
		fake.NewSimpleClientset(), &rest.Config{})

	// Act - set both streams to nil
	h.SetAttachStreams(nil, nil)

	// Assert - both should return EOF immediately without blocking
	done := make(chan struct{})
	go func() {
		gotStdout, _ := io.ReadAll(h.Stdout())
		gotStderr, _ := io.ReadAll(h.Stderr())
		if len(gotStdout) != 0 || len(gotStderr) != 0 {
			t.Errorf("Expected empty streams, got stdout=%q stderr=%q", gotStdout, gotStderr)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Reading from nil streams blocked - should return EOF immediately")
	}
}

// TestSetCompletedStreams_WaitChannelNonBlocking verifies that Wait() doesn't block
// and returns the correct result after SetCompletedStreams.
func TestSetCompletedStreams_WaitChannelNonBlocking(t *testing.T) {
	h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
		fake.NewSimpleClientset(), &rest.Config{})

	h.SetCompletedStreams("output", "", 42)

	// Wait should not block
	select {
	case result := <-h.Wait():
		if result.Code != 42 {
			t.Errorf("Wait result Code = %d, want 42", result.Code)
		}
	default:
		t.Error("Wait() blocked - should have result available immediately")
	}
}

// TestSetCompletedStreams_MultipleReads verifies that data can't be read twice
// (BufferedPipe consumes data on read).
func TestSetCompletedStreams_ReadConsumesData(t *testing.T) {
	h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
		fake.NewSimpleClientset(), &rest.Config{})

	h.SetCompletedStreams("test data", "", 0)

	// First read should get the data
	data1, err := io.ReadAll(h.Stdout())
	if err != nil {
		t.Fatalf("First read failed: %v", err)
	}
	if string(data1) != "test data" {
		t.Errorf("First read = %q, want %q", data1, "test data")
	}

	// Second read should get EOF (data consumed)
	data2, err := io.ReadAll(h.Stdout())
	if err != nil {
		t.Fatalf("Second read failed: %v", err)
	}
	if len(data2) != 0 {
		t.Errorf("Second read = %q, want empty (data should be consumed)", data2)
	}
}

// TestSignalExit_StatusTransitions verifies correct status for different exit scenarios.
func TestSignalExit_StatusTransitions(t *testing.T) {
	tests := []struct {
		name       string
		exitCode   int
		exitError  error
		wantStatus execbox.Status
	}{
		{
			name:       "exit 0 = stopped",
			exitCode:   0,
			exitError:  nil,
			wantStatus: execbox.StatusStopped,
		},
		{
			name:       "exit 1 = failed",
			exitCode:   1,
			exitError:  nil,
			wantStatus: execbox.StatusFailed,
		},
		{
			name:       "exit 137 (SIGKILL) = failed",
			exitCode:   137,
			exitError:  nil,
			wantStatus: execbox.StatusFailed,
		},
		{
			name:       "exit 0 with error = failed",
			exitCode:   0,
			exitError:  io.ErrUnexpectedEOF,
			wantStatus: execbox.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandle("test-id", "test-pod", "default", execbox.Spec{},
				fake.NewSimpleClientset(), &rest.Config{})

			h.SignalExit(execbox.ExitResult{
				Code:  tt.exitCode,
				Error: tt.exitError,
			})

			info := h.Info()
			if info.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", info.Status, tt.wantStatus)
			}
		})
	}
}

// Helper to truncate strings for error messages
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
