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
//   - Bidirectional streaming between client and backend
//
// Flow:
//  1. Extract session ID from path
//  2. Validate API key ownership of session
//  3. Verify session is running
//  4. Upgrade to WebSocket
//  5. Set up 4 goroutines for bidirectional I/O:
//     - Goroutine 1: WebSocket → stdin (client input to backend)
//     - Goroutine 2: stdout → WebSocket (backend output to client)
//     - Goroutine 3: stderr → WebSocket (backend errors to client)
//     - Goroutine 4: Wait for exit, send exit message
func (h *Handlers) AttachSession(w http.ResponseWriter, r *http.Request, sessionID string, apiKey *db.APIKey) {
	ctx := r.Context()

	// 1. Look up session in database
	session, err := h.db.GetSession(ctx, sessionID)
	if err != nil {
		WriteError(w, fmt.Errorf("session not found: %w", err), http.StatusNotFound, CodeNotFound)
		return
	}

	// 2. Verify ownership
	if session.APIKeyID != apiKey.ID {
		WriteError(w, fmt.Errorf("access denied"), http.StatusForbidden, CodeUnauthorized)
		return
	}

	// 3. Verify session status
	if session.Status != "running" && session.Status != "pending" {
		WriteError(w, fmt.Errorf("session not running: status=%s", session.Status), http.StatusConflict, CodeConflict)
		return
	}

	// 4. Get backend ID
	backendID := session.GetBackendID()
	if backendID == "" {
		WriteError(w, fmt.Errorf("session has no backend ID"), http.StatusInternalServerError, CodeInternal)
		return
	}

	// 5. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// upgrader.Upgrade writes the error response
		return
	}
	defer conn.Close()

	writer := &wsWriter{conn: conn}

	// 6. Attach to backend
	stdin, stdout, stderr, wait, err := h.backend.Attach(ctx, backendID)
	if err != nil {
		h.sendBinaryError(writer, fmt.Sprintf("failed to attach to session: %v", err))
		return
	}

	// 7. Handle bidirectional I/O using binary protocol
	h.handleBinaryProtocol(ctx, writer, conn, stdin, stdout, stderr, wait)
}

// handleBinaryProtocol manages bidirectional I/O using binary protocol
func (h *Handlers) handleBinaryProtocol(ctx context.Context, writer *wsWriter, conn *websocket.Conn, stdin io.WriteCloser, stdout io.Reader, stderr io.Reader, wait func() int) {
	// Use separate context for input (can be cancelled) vs output (must complete)
	inputCtx, cancelInput := context.WithCancel(ctx)
	defer cancelInput()

	var wg sync.WaitGroup
	var outputWg sync.WaitGroup

	// Goroutine 1: WebSocket → stdin (read WS, write to backend)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if stdin != nil {
			defer stdin.Close()
		}
		h.handleBinaryWSInput(inputCtx, conn, stdin, writer)
	}()

	// Goroutine 2: stdout → WebSocket (read backend, write WS)
	wg.Add(1)
	outputWg.Add(1)
	go func() {
		defer wg.Done()
		defer outputWg.Done()
		h.handleBinaryWSOutputNoCancel(writer, stdout, proto.MessageTypeStdout)
	}()

	// Goroutine 3: stderr → WebSocket
	wg.Add(1)
	outputWg.Add(1)
	go func() {
		defer wg.Done()
		defer outputWg.Done()
		h.handleBinaryWSOutputNoCancel(writer, stderr, proto.MessageTypeStderr)
	}()

	// Goroutine 4: Wait for exit, send exit message
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for output to complete
		outputWg.Wait()

		// Wait for session to exit and get exit code
		exitCode := wait()

		// Send exit message
		h.sendBinaryExit(writer, exitCode)
		cancelInput()
	}()

	// Wait for all goroutines to complete
	wg.Wait()
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
			// WebSocket closed normally or unexpectedly - either way, stop processing input
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
	_ = h.writeBinaryMessage(writer, msg)
}

// sendBinaryExit sends a binary exit message over WebSocket
func (h *Handlers) sendBinaryExit(writer *wsWriter, exitCode int) {
	msg := proto.BinaryMessage{
		Type:     proto.MessageTypeExit,
		ExitCode: exitCode,
	}
	_ = h.writeBinaryMessage(writer, msg)
}

// writeBinaryMessage writes a binary message to WebSocket
func (h *Handlers) writeBinaryMessage(writer *wsWriter, msg proto.BinaryMessage) error {
	data, err := (&proto.BinaryProtocol{}).Encode(msg)
	if err != nil {
		return err
	}

	return writer.WriteMessage(websocket.BinaryMessage, data)
}
