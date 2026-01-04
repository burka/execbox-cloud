package api

import (
	"encoding/json"
	"testing"
)

func TestCreateSessionRequestJSON(t *testing.T) {
	// Test that CreateSessionRequest matches the spec example
	req := CreateSessionRequest{
		Image:   "python:3.12",
		Command: []string{"python", "-c", "print('hello')"},
		Env:     map[string]string{"DEBUG": "1"},
		WorkDir: "/app",
		Resources: &Resources{
			CPUMillis: 1000,
			MemoryMB:  512,
			TimeoutMs: 30000,
		},
		Network: "exposed",
		Ports:   []PortSpec{{Container: 8080, Protocol: "tcp"}},
	}

	// Marshal to JSON
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Unmarshal back
	var decoded CreateSessionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Verify key fields
	if decoded.Image != "python:3.12" {
		t.Errorf("Expected image python:3.12, got %s", decoded.Image)
	}
	if decoded.Resources.MemoryMB != 512 {
		t.Errorf("Expected memory 512, got %d", decoded.Resources.MemoryMB)
	}
}

func TestCreateSessionResponseJSON(t *testing.T) {
	// Test that CreateSessionResponse matches the spec example
	resp := CreateSessionResponse{
		ID:        "sess_abc123",
		Status:    "running",
		CreatedAt: "2024-01-15T10:30:00Z",
		Network: &NetworkInfo{
			Mode: "exposed",
			Host: "localhost",
			Ports: map[string]PortInfo{
				"8080": {
					HostPort: 32789,
					URL:      "http://localhost:32789",
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Unmarshal back
	var decoded CreateSessionResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify key fields
	if decoded.ID != "sess_abc123" {
		t.Errorf("Expected ID sess_abc123, got %s", decoded.ID)
	}
	if decoded.Network.Ports["8080"].HostPort != 32789 {
		t.Errorf("Expected hostPort 32789, got %d", decoded.Network.Ports["8080"].HostPort)
	}
}

func TestErrorResponseJSON(t *testing.T) {
	// Test that ErrorResponse matches the spec example
	errResp := ErrorResponse{
		Error: "session not found",
		Code:  "NOT_FOUND",
	}

	// Marshal to JSON
	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	// Unmarshal back
	var decoded ErrorResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	// Verify fields
	if decoded.Error != "session not found" {
		t.Errorf("Expected error 'session not found', got %s", decoded.Error)
	}
	if decoded.Code != "NOT_FOUND" {
		t.Errorf("Expected code NOT_FOUND, got %s", decoded.Code)
	}
}

func TestSessionResponseOptionalFields(t *testing.T) {
	// Test that optional fields are omitted when nil
	resp := SessionResponse{
		ID:        "sess_123",
		Status:    "running",
		Image:     "alpine:3",
		CreatedAt: "2024-01-15T10:30:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Parse back as map to check omitted fields
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	// These fields should not be present
	if _, exists := m["startedAt"]; exists {
		t.Error("startedAt should be omitted when nil")
	}
	if _, exists := m["endedAt"]; exists {
		t.Error("endedAt should be omitted when nil")
	}
	if _, exists := m["exitCode"]; exists {
		t.Error("exitCode should be omitted when nil")
	}
	if _, exists := m["network"]; exists {
		t.Error("network should be omitted when nil")
	}
}

func TestGetURLResponse(t *testing.T) {
	resp := GetURLResponse{
		ContainerPort: 8080,
		HostPort:      32789,
		URL:           "http://localhost:32789",
		Protocol:      "tcp",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded GetURLResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if decoded.ContainerPort != 8080 {
		t.Errorf("Expected containerPort 8080, got %d", decoded.ContainerPort)
	}
	if decoded.URL != "http://localhost:32789" {
		t.Errorf("Expected URL http://localhost:32789, got %s", decoded.URL)
	}
}

func TestListDirectoryResponse(t *testing.T) {
	resp := ListDirectoryResponse{
		Path: "/app",
		Entries: []FileEntry{
			{Name: "main.py", Size: 1234, IsDir: false, Mode: 420},
			{Name: "data", Size: 0, IsDir: true, Mode: 493},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded ListDirectoryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(decoded.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(decoded.Entries))
	}
	if decoded.Entries[0].Name != "main.py" {
		t.Errorf("Expected first entry name main.py, got %s", decoded.Entries[0].Name)
	}
	if decoded.Entries[1].IsDir != true {
		t.Error("Expected second entry to be a directory")
	}
}
