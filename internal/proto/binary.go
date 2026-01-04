// Package proto provides binary protocol for WebSocket communication
package proto

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MessageType defines the type of WebSocket message
type MessageType byte

const (
	MessageTypeStdin      MessageType = 0x01
	MessageTypeStdout     MessageType = 0x02
	MessageTypeStderr     MessageType = 0x03
	MessageTypeExit       MessageType = 0x04
	MessageTypeError      MessageType = 0x05
	MessageTypeStdinClose MessageType = 0x06
)

// BinaryMessage represents a binary WebSocket message with type headers
type BinaryMessage struct {
	Type     MessageType
	Data     []byte
	ExitCode int    // For MessageTypeExit
	Error    string // For MessageTypeError
}

// BinaryProtocol handles encoding/decoding of binary WebSocket messages
type BinaryProtocol struct{}

// Encode converts a BinaryMessage to binary format
// Format: [Type(1 byte)][Length(4 bytes)][Data...][ExitCode(4 bytes)|ErrorLen(4 bytes)|Error...]
func (p *BinaryProtocol) Encode(msg BinaryMessage) ([]byte, error) {
	switch msg.Type {
	case MessageTypeStdin, MessageTypeStdout, MessageTypeStderr:
		// Data messages: [Type][Length][Data]
		dataLen := len(msg.Data)
		if dataLen > 1<<24-1 { // Max 16MB per message
			return nil, fmt.Errorf("message data too large: %d bytes", dataLen)
		}

		buf := make([]byte, 5+dataLen)
		buf[0] = byte(msg.Type)
		binary.BigEndian.PutUint32(buf[1:5], uint32(dataLen))
		copy(buf[5:], msg.Data)
		return buf, nil

	case MessageTypeExit:
		// Exit message: [Type][ExitCode]
		buf := make([]byte, 5)
		buf[0] = byte(msg.Type)
		binary.BigEndian.PutUint32(buf[1:5], uint32(msg.ExitCode))
		return buf, nil

	case MessageTypeError:
		// Error message: [Type][ErrorLen][Error]
		errorData := []byte(msg.Error)
		errorLen := len(errorData)
		if errorLen > 1024 { // Limit error message size
			errorData = errorData[:1024]
			errorLen = 1024
		}

		buf := make([]byte, 5+errorLen)
		buf[0] = byte(msg.Type)
		binary.BigEndian.PutUint32(buf[1:5], uint32(errorLen))
		copy(buf[5:], errorData)
		return buf, nil

	case MessageTypeStdinClose:
		// Close message: [Type] only
		return []byte{byte(msg.Type)}, nil

	default:
		return nil, fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

// Decode converts binary data to a BinaryMessage
func (p *BinaryProtocol) Decode(data []byte) (BinaryMessage, error) {
	if len(data) == 0 {
		return BinaryMessage{}, fmt.Errorf("empty message")
	}

	msgType := MessageType(data[0])
	switch msgType {
	case MessageTypeStdin, MessageTypeStdout, MessageTypeStderr:
		if len(data) < 5 {
			return BinaryMessage{}, fmt.Errorf("data message too short: %d bytes", len(data))
		}
		dataLen := binary.BigEndian.Uint32(data[1:5])
		if int(dataLen) != len(data)-5 {
			return BinaryMessage{}, fmt.Errorf("data length mismatch: expected %d, got %d", dataLen, len(data)-5)
		}
		if int(dataLen) > 1<<24-1 {
			return BinaryMessage{}, fmt.Errorf("data too large: %d bytes", dataLen)
		}

		msgData := make([]byte, dataLen)
		copy(msgData, data[5:])
		return BinaryMessage{
			Type: msgType,
			Data: msgData,
		}, nil

	case MessageTypeExit:
		if len(data) != 5 {
			return BinaryMessage{}, fmt.Errorf("exit message must be 5 bytes, got %d", len(data))
		}
		exitCode := int(binary.BigEndian.Uint32(data[1:5]))

		return BinaryMessage{
			Type:     msgType,
			ExitCode: exitCode,
		}, nil

	case MessageTypeError:
		if len(data) < 5 {
			return BinaryMessage{}, fmt.Errorf("error message too short: %d bytes", len(data))
		}
		errorLen := binary.BigEndian.Uint32(data[1:5])
		if int(errorLen) != len(data)-5 {
			return BinaryMessage{}, fmt.Errorf("error length mismatch: expected %d, got %d", errorLen, len(data)-5)
		}
		if int(errorLen) > 1024 {
			return BinaryMessage{}, fmt.Errorf("error message too large: %d bytes", errorLen)
		}

		errorMsg := string(data[5:])
		return BinaryMessage{
			Type:  msgType,
			Error: errorMsg,
		}, nil

	case MessageTypeStdinClose:
		if len(data) != 1 {
			return BinaryMessage{}, fmt.Errorf("close message must be 1 byte, got %d", len(data))
		}
		return BinaryMessage{Type: msgType}, nil

	default:
		return BinaryMessage{}, fmt.Errorf("unknown message type: %d", msgType)
	}
}

// WriteMessage writes a BinaryMessage to the writer
func (p *BinaryProtocol) WriteMessage(w io.Writer, msg BinaryMessage) error {
	data, err := p.Encode(msg)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// ReadMessage reads a BinaryMessage from the reader
func (p *BinaryProtocol) ReadMessage(r io.Reader) (BinaryMessage, error) {
	// Read message type
	header := make([]byte, 1)
	_, err := io.ReadFull(r, header)
	if err != nil {
		return BinaryMessage{}, err
	}

	msgType := MessageType(header[0])

	// Read remaining message based on type
	switch msgType {
	case MessageTypeStdin, MessageTypeStdout, MessageTypeStderr:
		// Read length (4 bytes)
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(r, lengthBuf)
		if err != nil {
			return BinaryMessage{}, err
		}
		dataLen := binary.BigEndian.Uint32(lengthBuf)

		// Validate length
		if dataLen > 1<<24-1 {
			return BinaryMessage{}, fmt.Errorf("data too large: %d bytes", dataLen)
		}

		// Read data
		data := make([]byte, dataLen)
		_, err = io.ReadFull(r, data)
		if err != nil {
			return BinaryMessage{}, err
		}

		return BinaryMessage{
			Type: msgType,
			Data: data,
		}, nil

	case MessageTypeExit:
		// Read exit code (4 bytes)
		codeBuf := make([]byte, 4)
		_, err := io.ReadFull(r, codeBuf)
		if err != nil {
			return BinaryMessage{}, err
		}
		exitCode := int(binary.BigEndian.Uint32(codeBuf))

		return BinaryMessage{
			Type:     msgType,
			ExitCode: exitCode,
		}, nil

	case MessageTypeError:
		// Read error length (4 bytes)
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(r, lengthBuf)
		if err != nil {
			return BinaryMessage{}, err
		}
		errorLen := binary.BigEndian.Uint32(lengthBuf)

		// Validate error length
		if errorLen > 1024 {
			return BinaryMessage{}, fmt.Errorf("error message too large: %d bytes", errorLen)
		}

		// Read error message
		errorData := make([]byte, errorLen)
		_, err = io.ReadFull(r, errorData)
		if err != nil {
			return BinaryMessage{}, err
		}

		return BinaryMessage{
			Type:  msgType,
			Error: string(errorData),
		}, nil

	case MessageTypeStdinClose:
		// No additional data
		return BinaryMessage{Type: msgType}, nil

	default:
		return BinaryMessage{}, fmt.Errorf("unknown message type: %d", msgType)
	}
}
