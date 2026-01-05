package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/burka/execbox-cloud/internal/proto"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins - WebSocket requests must include valid Bearer token
		// in the Authorization header, which browsers cannot send cross-origin.
		// This is safe because:
		// 1. Auth middleware validates the API key
		// 2. Browsers can't set Authorization header cross-origin
		// For additional security in browser contexts, restrict to specific origins.
		return true
	},
}

// wsWriter wraps a WebSocket connection to ensure thread-safe writes.
// gorilla/websocket only supports one concurrent writer at a time.
type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// WriteMessage safely writes a message to the WebSocket connection.
func (w *wsWriter) WriteMessage(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(messageType, data)
}

// AttachSession handles WebSocket attach endpoint: GET /v1/sessions/{id}/attach
//
// Protocol:
//   - Query parameter: protocol=binary|json (default: binary)
//   - Authentication: Bearer token (API key) in Authorization header
//   - Bidirectional streaming between client and Fly machine
//
// Flow:
//  1. Extract session ID from path
//  2. Validate API key ownership of session
//  3. Verify session is running
//  4. Upgrade to WebSocket
//  5. Set up 4 goroutines for bidirectional I/O:
//     - Goroutine 1: WebSocket → stdin (client input to machine)
//     - Goroutine 2: stdout → WebSocket (machine output to client)
//     - Goroutine 3: stderr → WebSocket (machine errors to client)
//     - Goroutine 4: Wait for exit, send exit message
func (h *Handlers) AttachSession(w http.ResponseWriter, r *http.Request, sessionID string, apiKey *db.APIKey) {
	// Feature not yet implemented - return clear error
	// TODO: Implement actual WebSocket attach when Fly machine I/O is ready
	WriteError(w, fmt.Errorf("session attach not yet implemented"), http.StatusNotImplemented, CodeNotImplemented)
}

// handleBinaryProtocol manages bidirectional I/O using binary protocol
func (h *Handlers) handleBinaryProtocol(ctx context.Context, writer *wsWriter, conn *websocket.Conn, session *db.Session) {
	// Use separate context for input (can be cancelled) vs output (must complete)
	inputCtx, cancelInput := context.WithCancel(ctx)
	defer cancelInput()

	var wg sync.WaitGroup
	var outputWg sync.WaitGroup

	// TODO: Get actual machine I/O streams from Fly.io
	// For now, using placeholder streams
	stdinWriter, stdoutReader, stderrReader := h.getMachineIOStreams(session)

	// Goroutine 1: WebSocket → stdin (read WS, write to machine)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if stdinWriter != nil {
			defer stdinWriter.Close()
		}
		h.handleBinaryWSInput(inputCtx, conn, stdinWriter, writer)
	}()

	// Goroutine 2: stdout → WebSocket (read machine, write WS)
	wg.Add(1)
	outputWg.Add(1)
	go func() {
		defer wg.Done()
		defer outputWg.Done()
		h.handleBinaryWSOutputNoCancel(writer, stdoutReader, proto.MessageTypeStdout)
	}()

	// Goroutine 3: stderr → WebSocket
	wg.Add(1)
	outputWg.Add(1)
	go func() {
		defer wg.Done()
		defer outputWg.Done()
		h.handleBinaryWSOutputNoCancel(writer, stderrReader, proto.MessageTypeStderr)
	}()

	// Goroutine 4: Wait for exit, send exit message
	wg.Add(1)
	go func() {
		defer wg.Done()
		// TODO: Wait for actual machine exit
		// For now, wait for output to complete
		outputWg.Wait()

		// Send exit message
		exitCode := 0 // TODO: Get actual exit code from machine
		h.sendBinaryExit(writer, exitCode)
		cancelInput()
	}()

	// Wait for all goroutines to complete
	wg.Wait()
}

// handleJSONProtocol manages bidirectional I/O using JSON protocol (future)
func (h *Handlers) handleJSONProtocol(ctx context.Context, writer *wsWriter, conn *websocket.Conn, session *db.Session) {
	// JSON protocol support can be added later
	// For now, send error message
	h.sendBinaryError(writer, "JSON protocol not yet implemented, use protocol=binary")
}

// getMachineIOStreams returns I/O streams for the Fly machine
// TODO: Replace with actual Fly.io machine connection
func (h *Handlers) getMachineIOStreams(session *db.Session) (io.WriteCloser, io.ReadCloser, io.ReadCloser) {
	// Placeholder streams - replace with actual Fly machine I/O
	// When Fly integration is ready, this will connect to the machine using session.FlyMachineID
	stdin := &nopWriteCloser{}
	stdout := &nopReadCloser{}
	stderr := &nopReadCloser{}
	return stdin, stdout, stderr
}

// handleBinaryWSInput reads binary WebSocket messages and writes to machine stdin
func (h *Handlers) handleBinaryWSInput(ctx context.Context, conn *websocket.Conn, stdin io.WriteCloser, writer *wsWriter) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read binary message from WebSocket
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				// Log unexpected close if needed
			}
			return
		}

		// Decode binary message
		msg, err := (&proto.BinaryProtocol{}).Decode(data)
		if err != nil {
			h.sendBinaryError(writer, fmt.Sprintf("invalid binary message: %v", err))
			continue
		}

		// Handle message types
		switch msg.Type {
		case proto.MessageTypeStdin:
			if stdin != nil {
				_, err = stdin.Write(msg.Data)
				if err != nil {
					h.sendBinaryError(writer, fmt.Sprintf("failed to write to stdin: %v", err))
					return
				}
			}
		case proto.MessageTypeStdinClose:
			if stdin != nil {
				stdin.Close()
			}
			return
		default:
			// Ignore other message types on input path
		}
	}
}

// handleBinaryWSOutputNoCancel reads from machine output stream and writes binary WebSocket messages
// This version doesn't check context cancellation, allowing it to drain all data
func (h *Handlers) handleBinaryWSOutputNoCancel(writer *wsWriter, reader io.Reader, msgType proto.MessageType) {
	if reader == nil {
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			msg := proto.BinaryMessage{
				Type: msgType,
				Data: buf[:n],
			}
			if err := h.writeBinaryMessage(writer, msg); err != nil {
				return
			}
		}

		if err != nil {
			if err != io.EOF {
				h.sendBinaryError(writer, fmt.Sprintf("error reading stream: %v", err))
			}
			return
		}
	}
}

// sendBinaryError sends a binary error message over WebSocket
func (h *Handlers) sendBinaryError(writer *wsWriter, message string) {
	msg := proto.BinaryMessage{
		Type:  proto.MessageTypeError,
		Error: message,
	}
	h.writeBinaryMessage(writer, msg)
}

// sendBinaryExit sends a binary exit message over WebSocket
func (h *Handlers) sendBinaryExit(writer *wsWriter, exitCode int) {
	msg := proto.BinaryMessage{
		Type:     proto.MessageTypeExit,
		ExitCode: exitCode,
	}
	h.writeBinaryMessage(writer, msg)
}

// writeBinaryMessage writes a binary message to WebSocket
func (h *Handlers) writeBinaryMessage(writer *wsWriter, msg proto.BinaryMessage) error {
	data, err := (&proto.BinaryProtocol{}).Encode(msg)
	if err != nil {
		return err
	}

	return writer.WriteMessage(websocket.BinaryMessage, data)
}

// Placeholder types for machine I/O streams
// TODO: Remove when Fly.io integration is ready

type nopWriteCloser struct{}

func (n *nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (n *nopWriteCloser) Close() error                { return nil }

type nopReadCloser struct{}

func (n *nopReadCloser) Read(p []byte) (int, error) { return 0, io.EOF }
func (n *nopReadCloser) Close() error               { return nil }
