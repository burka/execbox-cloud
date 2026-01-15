//nolint:staticcheck // fake.NewSimpleClientset is deprecated but fake.NewClientset requires generated apply configs
package k8s

import (
	"io"
	"testing"

	"github.com/burka/execbox/pkg/execbox"
	"k8s.io/client-go/kubernetes/fake"
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
		fake.NewSimpleClientset(),
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
		fake.NewSimpleClientset(),
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
		fake.NewSimpleClientset(),
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
		fake.NewSimpleClientset(),
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
		fake.NewSimpleClientset(),
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
		fake.NewSimpleClientset(),
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

func TestHandle_SetStatus(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Initial status should be Running
	if h.Info().Status != execbox.StatusRunning {
		t.Errorf("expected initial status Running, got %s", h.Info().Status)
	}

	// Act - Set to Stopped
	h.SetStatus(execbox.StatusStopped)

	// Assert
	info := h.Info()
	if info.Status != execbox.StatusStopped {
		t.Errorf("expected status Stopped, got %s", info.Status)
	}

	// Act - Set to Failed
	h.SetStatus(execbox.StatusFailed)

	// Assert
	info = h.Info()
	if info.Status != execbox.StatusFailed {
		t.Errorf("expected status Failed, got %s", info.Status)
	}
}

func TestHandle_SetExitError(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Initially no error
	info := h.Info()
	if info.Error != "" {
		t.Errorf("expected no error initially, got %s", info.Error)
	}

	// Act - Set exit error
	testErr := execbox.ErrSessionNotFound
	h.SetExitError(testErr)

	// Assert
	info = h.Info()
	if info.Error != testErr.Error() {
		t.Errorf("expected error '%s', got '%s'", testErr.Error(), info.Error)
	}

	// Act - Update with different error
	newErr := execbox.ErrTimeout
	h.SetExitError(newErr)

	// Assert
	info = h.Info()
	if info.Error != newErr.Error() {
		t.Errorf("expected error '%s', got '%s'", newErr.Error(), info.Error)
	}
}

func TestHandle_SetStdin(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Initially stdin returns a noop writer
	stdin := h.Stdin()
	n, err := stdin.Write([]byte("test"))
	if err != nil {
		t.Errorf("expected no error from noop writer, got %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 bytes written, got %d", n)
	}

	// Act - Set a real stdin pipe
	pr, pw := newTestPipe()
	defer pr.Close()
	defer pw.Close()

	h.SetStdin(pw)

	// Assert - Write through Stdin() should reach the pipe
	stdin = h.Stdin()
	testData := []byte("hello stdin")
	n, err = stdin.Write(testData)
	if err != nil {
		t.Errorf("expected no error writing to stdin, got %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected %d bytes written, got %d", len(testData), n)
	}

	// Verify data was written to the pipe
	buf := make([]byte, len(testData))
	n, err = pr.Read(buf)
	if err != nil {
		t.Errorf("expected no error reading from pipe, got %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected %d bytes read, got %d", len(testData), n)
	}
	if string(buf) != string(testData) {
		t.Errorf("expected data '%s', got '%s'", testData, buf)
	}
}

func TestHandle_SetNetwork(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Initially no network info
	info := h.Info()
	if info.Network != nil {
		t.Errorf("expected nil network initially, got %v", info.Network)
	}

	// Act - Set network info
	network := &execbox.NetworkInfo{
		Host: "localhost",
		Ports: map[int]execbox.PortInfo{
			8080: {HostPort: 8080},
		},
	}
	h.SetNetwork(network)

	// Assert
	info = h.Info()
	if info.Network == nil {
		t.Error("expected network to be set, got nil")
	} else {
		if info.Network.Host != "localhost" {
			t.Errorf("expected host 'localhost', got '%s'", info.Network.Host)
		}
		if len(info.Network.Ports) != 1 {
			t.Errorf("expected 1 port, got %d", len(info.Network.Ports))
		}
	}

	// Act - Update network info
	newNetwork := &execbox.NetworkInfo{
		Host: "127.0.0.1",
		Ports: map[int]execbox.PortInfo{
			3000: {HostPort: 3000},
			8080: {HostPort: 8080},
		},
	}
	h.SetNetwork(newNetwork)

	// Assert
	info = h.Info()
	if info.Network.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got '%s'", info.Network.Host)
	}
	if len(info.Network.Ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(info.Network.Ports))
	}
}

func TestHandle_SetCancelWatcher(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Act - Set cancel watcher
	called := false
	h.SetCancelWatcher(func() {
		called = true
	})

	// Assert - Cancel should be called on Close
	err := h.Close()
	if err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}
	if !called {
		t.Error("expected cancel watcher to be called on Close")
	}

	// Act - Close again should not panic (cancel already called)
	err = h.Close()
	if err != nil {
		t.Errorf("expected no error from second Close, got %v", err)
	}
}

func TestHandle_SetAttachStreams(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Create io.Pipe() for reliable testing
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	defer stdoutR.Close()
	defer stdoutW.Close()
	defer stderrR.Close()
	defer stderrW.Close()

	// Act - Set attach streams
	h.SetAttachStreams(stdoutR, stderrR)

	// Write to stdout in a goroutine (avoid blocking)
	testStdout := []byte("stdout data\n")
	go func() {
		_, _ = stdoutW.Write(testStdout)
		stdoutW.Close()
	}()

	// Assert - Read from handle's stdout
	buf := make([]byte, len(testStdout))
	n, err := h.Stdout().Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("expected no error reading stdout, got %v", err)
	}
	if n != len(testStdout) {
		t.Errorf("expected %d bytes from stdout, got %d", len(testStdout), n)
	}
	if string(buf[:n]) != string(testStdout) {
		t.Errorf("expected stdout '%s', got '%s'", testStdout, buf[:n])
	}

	// Write to stderr in a goroutine (avoid blocking)
	testStderr := []byte("stderr data\n")
	go func() {
		_, _ = stderrW.Write(testStderr)
		stderrW.Close()
	}()

	// Assert - Read from handle's stderr
	buf = make([]byte, len(testStderr))
	n, err = h.Stderr().Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("expected no error reading stderr, got %v", err)
	}
	if n != len(testStderr) {
		t.Errorf("expected %d bytes from stderr, got %d", len(testStderr), n)
	}
	if string(buf[:n]) != string(testStderr) {
		t.Errorf("expected stderr '%s', got '%s'", testStderr, buf[:n])
	}
}

func TestHandle_ConcurrentOps(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	// Act - Run concurrent operations
	done := make(chan bool)
	iterations := 100

	// Goroutine 1: SetStatus
	go func() {
		for i := 0; i < iterations; i++ {
			h.SetStatus(execbox.StatusRunning)
			h.SetStatus(execbox.StatusStopping)
		}
		done <- true
	}()

	// Goroutine 2: Info
	go func() {
		for i := 0; i < iterations; i++ {
			_ = h.Info()
		}
		done <- true
	}()

	// Goroutine 3: SetExitCode and SetExitError
	go func() {
		for i := 0; i < iterations; i++ {
			h.SetExitCode(i)
			h.SetExitError(execbox.ErrSessionNotFound)
		}
		done <- true
	}()

	// Goroutine 4: SetNetwork
	go func() {
		for i := 0; i < iterations; i++ {
			network := &execbox.NetworkInfo{
				Host: "localhost",
				Ports: map[int]execbox.PortInfo{
					8080: {HostPort: 8080},
				},
			}
			h.SetNetwork(network)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// Assert - Handle should still be in a valid state
	info := h.Info()
	if info.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", info.ID)
	}

	// Close should not panic
	err := h.Close()
	if err != nil {
		t.Errorf("expected no error from Close after concurrent ops, got %v", err)
	}
}

func TestHandle_CloseWithStdin(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	pr, pw := io.Pipe()
	defer pr.Close()

	h.SetStdin(pw)

	// Get stdin reference before close
	stdin := h.Stdin()

	// Act - Close handle (should close the underlying stdin pipe)
	err := h.Close()

	// Assert
	if err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}

	// Writing to the original stdin reference after close should fail
	// because Close() closed the underlying pipe
	_, writeErr := stdin.Write([]byte("should fail"))
	if writeErr == nil {
		t.Error("expected error writing to closed stdin, got nil")
	}
}

func TestHandle_CloseWithAttachStreams(t *testing.T) {
	// Arrange
	h := NewHandle(
		"test-id",
		"test-pod",
		"default",
		execbox.Spec{},
		fake.NewSimpleClientset(),
		&rest.Config{},
	)

	stdoutR, stdoutW := newTestPipe()
	stderrR, stderrW := newTestPipe()
	defer stdoutW.Close()
	defer stderrW.Close()

	h.SetAttachStreams(stdoutR, stderrR)

	// Act - Close handle
	err := h.Close()

	// Assert
	if err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}

	// Attach streams should be closed
	// We can't directly test if they're closed, but Close should not panic
}

func TestHandle_NoopWriteCloser(t *testing.T) {
	// Arrange
	noop := &noopWriteCloser{}

	// Act & Assert - Write should succeed
	testData := []byte("test data")
	n, err := noop.Write(testData)
	if err != nil {
		t.Errorf("expected no error from noop Write, got %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected %d bytes written, got %d", len(testData), n)
	}

	// Act & Assert - Close should succeed
	err = noop.Close()
	if err != nil {
		t.Errorf("expected no error from noop Close, got %v", err)
	}
}

func TestHandle_ReadyWriter(t *testing.T) {
	// Arrange
	readyChan := make(chan struct{}, 1)
	writer := &readyWriter{readyChan: readyChan}

	// Act
	testData := []byte("test data")
	n, err := writer.Write(testData)

	// Assert
	if err != nil {
		t.Errorf("expected no error from readyWriter Write, got %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected %d bytes written, got %d", len(testData), n)
	}
}

// Helper function to create test pipes
func newTestPipe() (*testPipeReader, *testPipeWriter) {
	pr, pw := newPipe()
	return pr, pw
}

// testPipe implements a simple in-memory pipe for testing
type testPipeReader struct {
	data chan []byte
	err  chan error
}

type testPipeWriter struct {
	data chan []byte
	err  chan error
}

func newPipe() (*testPipeReader, *testPipeWriter) {
	data := make(chan []byte, 10)
	err := make(chan error, 1)
	return &testPipeReader{data: data, err: err}, &testPipeWriter{data: data, err: err}
}

func (r *testPipeReader) Read(p []byte) (int, error) {
	select {
	case data := <-r.data:
		copy(p, data)
		return len(data), nil
	case err := <-r.err:
		return 0, err
	}
}

func (r *testPipeReader) Close() error {
	select {
	case r.err <- io.ErrClosedPipe:
	default:
	}
	return nil
}

func (w *testPipeWriter) Write(p []byte) (int, error) {
	data := make([]byte, len(p))
	copy(data, p)
	select {
	case w.data <- data:
		return len(p), nil
	case err := <-w.err:
		return 0, err
	}
}

func (w *testPipeWriter) Close() error {
	select {
	case w.err <- io.ErrClosedPipe:
	default:
	}
	return nil
}
