package fly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	client := New("test-token", "test-org", "test-app")

	if client.token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", client.token)
	}
	if client.org != "test-org" {
		t.Errorf("expected org 'test-org', got '%s'", client.org)
	}
	if client.appName != "test-app" {
		t.Errorf("expected appName 'test-app', got '%s'", client.appName)
	}
	if client.baseURL != defaultBaseURL {
		t.Errorf("expected baseURL '%s', got '%s'", defaultBaseURL, client.baseURL)
	}
}

func TestClientWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	client := New("token", "org", "app").WithHTTPClient(customClient)

	if client.httpClient != customClient {
		t.Error("expected custom HTTP client to be set")
	}
}

func TestClientWithBaseURL(t *testing.T) {
	customURL := "https://custom.example.com"
	client := New("token", "org", "app").WithBaseURL(customURL)

	if client.baseURL != customURL {
		t.Errorf("expected baseURL '%s', got '%s'", customURL, client.baseURL)
	}
}

func TestCreateMachine(t *testing.T) {
	tests := []struct {
		name           string
		config         *MachineConfig
		serverResponse string
		statusCode     int
		wantError      bool
		wantMachineID  string
	}{
		{
			name: "successful creation",
			config: &MachineConfig{
				Image: "nginx:latest",
				Env:   map[string]string{"FOO": "bar"},
			},
			serverResponse: `{
				"id": "machine-123",
				"name": "test-machine",
				"state": "created",
				"region": "iad",
				"created_at": "2024-01-01T00:00:00Z",
				"config": {
					"image": "nginx:latest",
					"env": {"FOO": "bar"}
				}
			}`,
			statusCode:    http.StatusOK,
			wantError:     false,
			wantMachineID: "machine-123",
		},
		{
			name:           "nil config",
			config:         nil,
			serverResponse: "",
			statusCode:     http.StatusOK,
			wantError:      true,
		},
		{
			name: "server error",
			config: &MachineConfig{
				Image: "nginx:latest",
			},
			serverResponse: `{"error": "internal server error"}`,
			statusCode:     http.StatusInternalServerError,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/apps/test-app/machines") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("expected Authorization header 'Bearer test-token', got '%s'", auth)
				}

				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			}))
			defer server.Close()

			client := New("test-token", "test-org", "test-app").WithBaseURL(server.URL)
			machine, err := client.CreateMachine(context.Background(), tt.config)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if machine.ID != tt.wantMachineID {
				t.Errorf("expected machine ID '%s', got '%s'", tt.wantMachineID, machine.ID)
			}
		})
	}
}

func TestGetMachine(t *testing.T) {
	tests := []struct {
		name           string
		machineID      string
		serverResponse string
		statusCode     int
		wantError      bool
		wantState      string
	}{
		{
			name:      "successful get",
			machineID: "machine-123",
			serverResponse: `{
				"id": "machine-123",
				"name": "test-machine",
				"state": "started",
				"region": "iad",
				"created_at": "2024-01-01T00:00:00Z"
			}`,
			statusCode: http.StatusOK,
			wantError:  false,
			wantState:  "started",
		},
		{
			name:           "empty machine ID",
			machineID:      "",
			serverResponse: "",
			statusCode:     http.StatusOK,
			wantError:      true,
		},
		{
			name:           "not found",
			machineID:      "nonexistent",
			serverResponse: `{"error": "machine not found"}`,
			statusCode:     http.StatusNotFound,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("expected GET request, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			}))
			defer server.Close()

			client := New("test-token", "test-org", "test-app").WithBaseURL(server.URL)
			machine, err := client.GetMachine(context.Background(), tt.machineID)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if machine.State != tt.wantState {
				t.Errorf("expected state '%s', got '%s'", tt.wantState, machine.State)
			}
		})
	}
}

func TestStartMachine(t *testing.T) {
	tests := []struct {
		name       string
		machineID  string
		statusCode int
		wantError  bool
	}{
		{
			name:       "successful start",
			machineID:  "machine-123",
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "empty machine ID",
			machineID:  "",
			statusCode: http.StatusOK,
			wantError:  true,
		},
		{
			name:       "server error",
			machineID:  "machine-123",
			statusCode: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/start") {
					t.Errorf("expected path to end with /start, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := New("test-token", "test-org", "test-app").WithBaseURL(server.URL)
			err := client.StartMachine(context.Background(), tt.machineID)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestStopMachine(t *testing.T) {
	tests := []struct {
		name       string
		machineID  string
		statusCode int
		wantError  bool
	}{
		{
			name:       "successful stop",
			machineID:  "machine-123",
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "empty machine ID",
			machineID:  "",
			statusCode: http.StatusOK,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/stop") {
					t.Errorf("expected path to end with /stop, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := New("test-token", "test-org", "test-app").WithBaseURL(server.URL)
			err := client.StopMachine(context.Background(), tt.machineID)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDestroyMachine(t *testing.T) {
	tests := []struct {
		name       string
		machineID  string
		statusCode int
		wantError  bool
	}{
		{
			name:       "successful destroy",
			machineID:  "machine-123",
			statusCode: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "empty machine ID",
			machineID:  "",
			statusCode: http.StatusOK,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("expected DELETE request, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := New("test-token", "test-org", "test-app").WithBaseURL(server.URL)
			err := client.DestroyMachine(context.Background(), tt.machineID)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestWaitForState(t *testing.T) {
	tests := []struct {
		name         string
		machineID    string
		desiredState string
		states       []string // Sequence of states to return
		timeout      time.Duration
		wantError    bool
	}{
		{
			name:         "successful state transition",
			machineID:    "machine-123",
			desiredState: "started",
			states:       []string{"created", "starting", "started"},
			timeout:      5 * time.Second,
			wantError:    false,
		},
		{
			name:         "timeout waiting for state",
			machineID:    "machine-123",
			desiredState: "started",
			states:       []string{"created", "created", "created"},
			timeout:      2 * time.Second,
			wantError:    true,
		},
		{
			name:         "empty machine ID",
			machineID:    "",
			desiredState: "started",
			states:       []string{},
			timeout:      1 * time.Second,
			wantError:    true,
		},
		{
			name:         "empty desired state",
			machineID:    "machine-123",
			desiredState: "",
			states:       []string{},
			timeout:      1 * time.Second,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateIndex := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var state string
				if stateIndex < len(tt.states) {
					state = tt.states[stateIndex]
					stateIndex++
				} else {
					state = tt.states[len(tt.states)-1]
				}

				response := Machine{
					ID:    tt.machineID,
					State: state,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			client := New("test-token", "test-org", "test-app").WithBaseURL(server.URL)
			err := client.WaitForState(context.Background(), tt.machineID, tt.desiredState, tt.timeout)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRateLimitRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"error": "rate limited"}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
			return
		}

		// Success on third attempt
		w.WriteHeader(http.StatusOK)
		response := Machine{
			ID:    "machine-123",
			State: "started",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := New("test-token", "test-org", "test-app").
		WithBaseURL(server.URL).
		WithHTTPClient(&http.Client{Timeout: 10 * time.Second})

	machine, err := client.GetMachine(context.Background(), "machine-123")
	if err != nil {
		t.Errorf("expected successful retry after rate limit, got error: %v", err)
	}
	if machine.ID != "machine-123" {
		t.Errorf("expected machine ID 'machine-123', got '%s'", machine.ID)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRateLimitExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		if _, err := w.Write([]byte(`{"error": "rate limited"}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := New("test-token", "test-org", "test-app").
		WithBaseURL(server.URL).
		WithHTTPClient(&http.Client{Timeout: 10 * time.Second})

	_, err := client.GetMachine(context.Background(), "machine-123")
	if err == nil {
		t.Error("expected error after exceeding rate limit retries, got nil")
	}

	flyErr, ok := err.(*FlyError)
	if !ok {
		t.Errorf("expected FlyError, got %T", err)
	}
	if !flyErr.IsRateLimited() {
		t.Error("expected IsRateLimited to be true")
	}
}

func TestFlyError(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		message         string
		wantNotFound    bool
		wantRateLimited bool
	}{
		{
			name:            "404 not found",
			statusCode:      404,
			message:         "not found",
			wantNotFound:    true,
			wantRateLimited: false,
		},
		{
			name:            "429 rate limited",
			statusCode:      429,
			message:         "too many requests",
			wantNotFound:    false,
			wantRateLimited: true,
		},
		{
			name:            "500 server error",
			statusCode:      500,
			message:         "internal server error",
			wantNotFound:    false,
			wantRateLimited: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &FlyError{
				StatusCode: tt.statusCode,
				Message:    tt.message,
			}

			if err.IsNotFound() != tt.wantNotFound {
				t.Errorf("IsNotFound() = %v, want %v", err.IsNotFound(), tt.wantNotFound)
			}
			if err.IsRateLimited() != tt.wantRateLimited {
				t.Errorf("IsRateLimited() = %v, want %v", err.IsRateLimited(), tt.wantRateLimited)
			}

			errStr := err.Error()
			if !strings.Contains(errStr, tt.message) {
				t.Errorf("Error() should contain message '%s', got '%s'", tt.message, errStr)
			}
		})
	}
}
