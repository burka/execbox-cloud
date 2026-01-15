package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/burka/execbox-cloud/internal/proto"
	"github.com/gorilla/websocket"
)

// Note: mockDB is already defined in middleware_test.go
// We just test the basic functionality here without full mocking

func TestWSWriter_ThreadSafety(t *testing.T) {
	// Test that wsWriter is thread-safe
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		writer := &wsWriter{conn: conn}

		// Write from multiple goroutines concurrently
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				bp := &proto.BinaryProtocol{}
				msg := proto.BinaryMessage{
					Type: proto.MessageTypeStdout,
					Data: []byte{byte(n)},
				}
				data, _ := bp.Encode(msg)
				_ = writer.WriteMessage(websocket.BinaryMessage, data)
			}(i)
		}

		wg.Wait()
		time.Sleep(100 * time.Millisecond) // Give client time to read
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Read messages
	messagesReceived := 0
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	for i := 0; i < 10; i++ {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		messagesReceived++
	}

	if messagesReceived == 0 {
		t.Error("expected to receive at least some messages")
	}
}

func TestBinaryProtocolEncoding(t *testing.T) {
	bp := &proto.BinaryProtocol{}

	tests := []struct {
		name string
		msg  proto.BinaryMessage
	}{
		{
			name: "stdin message",
			msg: proto.BinaryMessage{
				Type: proto.MessageTypeStdin,
				Data: []byte("hello world"),
			},
		},
		{
			name: "stdout message",
			msg: proto.BinaryMessage{
				Type: proto.MessageTypeStdout,
				Data: []byte("output data"),
			},
		},
		{
			name: "exit message",
			msg: proto.BinaryMessage{
				Type:     proto.MessageTypeExit,
				ExitCode: 42,
			},
		},
		{
			name: "error message",
			msg: proto.BinaryMessage{
				Type:  proto.MessageTypeError,
				Error: "test error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := bp.Encode(tt.msg)
			if err != nil {
				t.Fatalf("failed to encode: %v", err)
			}

			// Decode
			decoded, err := bp.Decode(data)
			if err != nil {
				t.Fatalf("failed to decode: %v", err)
			}

			// Verify
			if decoded.Type != tt.msg.Type {
				t.Errorf("type mismatch: got %v, want %v", decoded.Type, tt.msg.Type)
			}

			if string(decoded.Data) != string(tt.msg.Data) {
				t.Errorf("data mismatch: got %q, want %q", decoded.Data, tt.msg.Data)
			}

			if decoded.ExitCode != tt.msg.ExitCode {
				t.Errorf("exit code mismatch: got %v, want %v", decoded.ExitCode, tt.msg.ExitCode)
			}

			if decoded.Error != tt.msg.Error {
				t.Errorf("error mismatch: got %q, want %q", decoded.Error, tt.msg.Error)
			}
		})
	}
}

func TestUpgrader_CheckOrigin(t *testing.T) {
	// Verify that the upgrader allows all origins
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")

	allowed := upgrader.CheckOrigin(req)
	if !allowed {
		t.Error("expected CheckOrigin to return true for all origins")
	}
}

func TestPlaceholderStreams(t *testing.T) {
	// Test the placeholder I/O streams
	stdin := &nopWriteCloser{}
	n, err := stdin.Write([]byte("test"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 bytes written, got %d", n)
	}
	if err := stdin.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}

	stdout := &nopReadCloser{}
	buf := make([]byte, 10)
	n, err = stdout.Read(buf)
	if n != 0 {
		t.Errorf("expected 0 bytes read, got %d", n)
	}
	if err == nil {
		t.Error("expected EOF error")
	}
	if err := stdout.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

// Integration test demonstrating the WebSocket attach flow
func TestWebSocketAttachFlow(t *testing.T) {
	t.Skip("Skipping integration test - requires database and Fly.io setup")

	// This would be a full integration test with actual database
	// For now, we skip it as it requires infrastructure
}

// Verify handler creation
func TestNewHandlers_Integration(t *testing.T) {
	// This test verifies that NewHandlers from handlers.go works correctly
	// with the websocket code

	handlers := NewHandlers(nil, nil)
	if handlers == nil {
		t.Fatal("NewHandlers returned nil")
	}

	// Verify handlers has the AttachSession method
	// This is a compile-time check that the method exists
	var _ func(http.ResponseWriter, *http.Request, string, *db.APIKey) = handlers.AttachSession
}

// Test the handlers structure
func TestHandlersStructure(t *testing.T) {
	// Create handlers with nil dependencies
	h := &Handlers{
		db:  nil,
		fly: nil,
	}

	// Verify we can call getMachineIOStreams (even though it returns placeholders)
	stdin, stdout, stderr := h.getMachineIOStreams(&db.Session{})
	if stdin == nil || stdout == nil || stderr == nil {
		t.Error("expected non-nil I/O streams")
	}
}

// Benchmark wsWriter thread safety
func BenchmarkWSWriter_Concurrent(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		writer := &wsWriter{conn: conn}

		for i := 0; i < b.N; i++ {
			bp := &proto.BinaryProtocol{}
			msg := proto.BinaryMessage{
				Type: proto.MessageTypeStdout,
				Data: []byte{byte(i % 256)},
			}
			data, _ := bp.Encode(msg)
			_ = writer.WriteMessage(websocket.BinaryMessage, data)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	b.ResetTimer()

	// Read messages in background
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	time.Sleep(time.Second)
}

// Ensure Handlers satisfies expected interface
func TestHandlers_HasRequiredMethods(t *testing.T) {
	h := &Handlers{}

	// Compile-time verification that these methods exist
	_ = h.AttachSession
	_ = h.getMachineIOStreams
	_ = h.handleBinaryProtocol
	_ = h.handleJSONProtocol
	_ = h.handleBinaryWSInput
	_ = h.handleBinaryWSOutputNoCancel
	_ = h.sendBinaryError
	_ = h.sendBinaryExit
	_ = h.writeBinaryMessage
}

// Test that fly client is available in handlers
func TestHandlers_FlyClient(t *testing.T) {
	flyClient := fly.New("test-token", "test-org", "test-app")
	handlers := NewHandlers(nil, flyClient)

	if handlers == nil {
		t.Fatal("handlers is nil")
	}

	// The fly client should be available for future machine I/O integration
}
