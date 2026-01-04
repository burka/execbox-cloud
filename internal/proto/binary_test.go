package proto

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeStdout(t *testing.T) {
	protocol := &BinaryProtocol{}

	msg := BinaryMessage{
		Type: MessageTypeStdout,
		Data: []byte("Hello, World!"),
	}

	encoded, err := protocol.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := protocol.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected %v, got %v", msg.Type, decoded.Type)
	}

	if len(decoded.Data) != len(msg.Data) {
		t.Errorf("Data length mismatch: expected %d, got %d", len(msg.Data), len(decoded.Data))
	}
}

func TestEncodeDecodeExit(t *testing.T) {
	protocol := &BinaryProtocol{}

	msg := BinaryMessage{
		Type:     MessageTypeExit,
		ExitCode: 42,
	}

	encoded, err := protocol.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := protocol.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected %v, got %v", msg.Type, decoded.Type)
	}

	if decoded.ExitCode != msg.ExitCode {
		t.Errorf("ExitCode mismatch: expected %d, got %d", msg.ExitCode, decoded.ExitCode)
	}
}

func TestRoundTripAllMessageTypes(t *testing.T) {
	protocol := &BinaryProtocol{}

	testCases := []struct {
		name string
		msg  BinaryMessage
	}{
		{
			name: "stdin",
			msg: BinaryMessage{
				Type: MessageTypeStdin,
				Data: []byte("input data"),
			},
		},
		{
			name: "stdout",
			msg: BinaryMessage{
				Type: MessageTypeStdout,
				Data: []byte("output data"),
			},
		},
		{
			name: "stderr",
			msg: BinaryMessage{
				Type: MessageTypeStderr,
				Data: []byte("error output"),
			},
		},
		{
			name: "exit_success",
			msg: BinaryMessage{
				Type:     MessageTypeExit,
				ExitCode: 0,
			},
		},
		{
			name: "exit_failure",
			msg: BinaryMessage{
				Type:     MessageTypeExit,
				ExitCode: 1,
			},
		},
		{
			name: "error",
			msg: BinaryMessage{
				Type:  MessageTypeError,
				Error: "something went wrong",
			},
		},
		{
			name: "stdin_close",
			msg: BinaryMessage{
				Type: MessageTypeStdinClose,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := protocol.Encode(tc.msg)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded, err := protocol.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if decoded.Type != tc.msg.Type {
				t.Errorf("Type mismatch: expected %v, got %v", tc.msg.Type, decoded.Type)
			}

			// Check type-specific fields
			switch tc.msg.Type {
			case MessageTypeStdin, MessageTypeStdout, MessageTypeStderr:
				if !bytes.Equal(decoded.Data, tc.msg.Data) {
					t.Errorf("Data mismatch: expected %v, got %v", tc.msg.Data, decoded.Data)
				}
			case MessageTypeExit:
				if decoded.ExitCode != tc.msg.ExitCode {
					t.Errorf("ExitCode mismatch: expected %d, got %d", tc.msg.ExitCode, decoded.ExitCode)
				}
			case MessageTypeError:
				if decoded.Error != tc.msg.Error {
					t.Errorf("Error mismatch: expected %s, got %s", tc.msg.Error, decoded.Error)
				}
			}
		})
	}
}

func TestEncodeDecodeEmptyData(t *testing.T) {
	protocol := &BinaryProtocol{}

	msg := BinaryMessage{
		Type: MessageTypeStdout,
		Data: []byte{},
	}

	encoded, err := protocol.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := protocol.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected %v, got %v", msg.Type, decoded.Type)
	}

	if len(decoded.Data) != 0 {
		t.Errorf("Expected empty data, got %d bytes", len(decoded.Data))
	}
}

func TestEncodeDataTooLarge(t *testing.T) {
	protocol := &BinaryProtocol{}

	largeData := make([]byte, 1<<24) // Exceeds max size
	msg := BinaryMessage{
		Type: MessageTypeStdout,
		Data: largeData,
	}

	_, err := protocol.Encode(msg)
	if err == nil {
		t.Error("Expected error for data too large, got nil")
	}
}

func TestDecodeEmptyMessage(t *testing.T) {
	protocol := &BinaryProtocol{}

	_, err := protocol.Decode([]byte{})
	if err == nil {
		t.Error("Expected error for empty message, got nil")
	}
}

func TestDecodeInvalidMessageType(t *testing.T) {
	protocol := &BinaryProtocol{}

	data := []byte{0xFF, 0x00, 0x00, 0x00, 0x00}
	_, err := protocol.Decode(data)
	if err == nil {
		t.Error("Expected error for invalid message type, got nil")
	}
}

func TestDecodeDataMessageTooShort(t *testing.T) {
	protocol := &BinaryProtocol{}

	data := []byte{byte(MessageTypeStdout), 0x00, 0x00} // Missing length bytes
	_, err := protocol.Decode(data)
	if err == nil {
		t.Error("Expected error for short data message, got nil")
	}
}

func TestDecodeDataLengthMismatch(t *testing.T) {
	protocol := &BinaryProtocol{}

	// Message says length is 10, but actual data is 5
	data := []byte{byte(MessageTypeStdout), 0x00, 0x00, 0x00, 0x0A, 0x01, 0x02, 0x03, 0x04, 0x05}
	_, err := protocol.Decode(data)
	if err == nil {
		t.Error("Expected error for data length mismatch, got nil")
	}
}

func TestDecodeExitWrongSize(t *testing.T) {
	protocol := &BinaryProtocol{}

	data := []byte{byte(MessageTypeExit), 0x00, 0x00} // Should be 5 bytes total
	_, err := protocol.Decode(data)
	if err == nil {
		t.Error("Expected error for wrong exit message size, got nil")
	}
}

func TestDecodeStdinCloseWrongSize(t *testing.T) {
	protocol := &BinaryProtocol{}

	data := []byte{byte(MessageTypeStdinClose), 0x00} // Should be 1 byte total
	_, err := protocol.Decode(data)
	if err == nil {
		t.Error("Expected error for wrong close message size, got nil")
	}
}

func TestErrorMessageTruncation(t *testing.T) {
	protocol := &BinaryProtocol{}

	// Create error message longer than 1024 bytes
	longError := string(make([]byte, 2000))
	msg := BinaryMessage{
		Type:  MessageTypeError,
		Error: longError,
	}

	encoded, err := protocol.Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify encoded message is truncated to max size (1 + 4 + 1024)
	expectedSize := 1 + 4 + 1024
	if len(encoded) != expectedSize {
		t.Errorf("Expected encoded size %d, got %d", expectedSize, len(encoded))
	}
}

func TestWriteReadMessage(t *testing.T) {
	protocol := &BinaryProtocol{}
	buf := &bytes.Buffer{}

	msg := BinaryMessage{
		Type: MessageTypeStdout,
		Data: []byte("test data"),
	}

	err := protocol.WriteMessage(buf, msg)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	decoded, err := protocol.ReadMessage(buf)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected %v, got %v", msg.Type, decoded.Type)
	}

	if !bytes.Equal(decoded.Data, msg.Data) {
		t.Errorf("Data mismatch: expected %v, got %v", msg.Data, decoded.Data)
	}
}

func TestReadMessageAllTypes(t *testing.T) {
	protocol := &BinaryProtocol{}

	testCases := []BinaryMessage{
		{Type: MessageTypeStdin, Data: []byte("stdin test")},
		{Type: MessageTypeStdout, Data: []byte("stdout test")},
		{Type: MessageTypeStderr, Data: []byte("stderr test")},
		{Type: MessageTypeExit, ExitCode: 123},
		{Type: MessageTypeError, Error: "error test"},
		{Type: MessageTypeStdinClose},
	}

	for _, tc := range testCases {
		buf := &bytes.Buffer{}

		err := protocol.WriteMessage(buf, tc)
		if err != nil {
			t.Fatalf("WriteMessage failed for type %v: %v", tc.Type, err)
		}

		decoded, err := protocol.ReadMessage(buf)
		if err != nil {
			t.Fatalf("ReadMessage failed for type %v: %v", tc.Type, err)
		}

		if decoded.Type != tc.Type {
			t.Errorf("Type mismatch: expected %v, got %v", tc.Type, decoded.Type)
		}
	}
}
