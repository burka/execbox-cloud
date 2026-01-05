package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// mockHandlerDB is a mock implementation of the database client for handler tests
type mockHandlerDB struct {
	sessions        map[string]*db.Session
	createErr       error
	getErr          error
	listErr         error
	updateErr       error
	deleteErr       error
	apiKeysByString map[string]*db.APIKey
	lastUsedCalls   map[uuid.UUID]int
}

func newMockHandlerDB() *mockHandlerDB {
	return &mockHandlerDB{
		sessions:        make(map[string]*db.Session),
		apiKeysByString: make(map[string]*db.APIKey),
		lastUsedCalls:   make(map[uuid.UUID]int),
	}
}

func (m *mockHandlerDB) CreateSession(ctx context.Context, session *db.Session) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockHandlerDB) GetSession(ctx context.Context, id string) (*db.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

func (m *mockHandlerDB) ListSessions(ctx context.Context, apiKeyID uuid.UUID, status *string) ([]db.Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	var sessions []db.Session
	for _, session := range m.sessions {
		if session.APIKeyID == apiKeyID {
			if status == nil || session.Status == *status {
				sessions = append(sessions, *session)
			}
		}
	}
	return sessions, nil
}

func (m *mockHandlerDB) UpdateSession(ctx context.Context, id string, update *db.SessionUpdate) error {
	if m.updateErr != nil {
		return m.updateErr
	}

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found")
	}

	if update.Status != nil {
		session.Status = *update.Status
	}
	if update.FlyMachineID != nil {
		session.FlyMachineID = update.FlyMachineID
	}
	if update.FlyAppID != nil {
		session.FlyAppID = update.FlyAppID
	}
	if update.ExitCode != nil {
		session.ExitCode = update.ExitCode
	}
	if update.Ports != nil {
		session.Ports = update.Ports
	}
	if update.StartedAt != nil {
		session.StartedAt = update.StartedAt
	}
	if update.EndedAt != nil {
		session.EndedAt = update.EndedAt
	}

	return nil
}

func (m *mockHandlerDB) DeleteSession(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found")
	}
	delete(m.sessions, id)
	return nil
}

func (m *mockHandlerDB) GetAPIKeyByKey(ctx context.Context, key string) (*db.APIKey, error) {
	apiKey, ok := m.apiKeysByString[key]
	if !ok {
		return nil, fmt.Errorf("API key not found")
	}
	return apiKey, nil
}

func (m *mockHandlerDB) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	m.lastUsedCalls[id]++
	return nil
}

func (m *mockHandlerDB) GetActiveSessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error) {
	count := 0
	for _, session := range m.sessions {
		if session.APIKeyID == apiKeyID && (session.Status == "running" || session.Status == "pending") {
			count++
		}
	}
	return count, nil
}

func (m *mockHandlerDB) GetDailySessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error) {
	count := 0
	for _, session := range m.sessions {
		if session.APIKeyID == apiKeyID {
			count++
		}
	}
	return count, nil
}

func (m *mockHandlerDB) CreateQuotaRequest(ctx context.Context, req *db.QuotaRequest) (*db.QuotaRequest, error) {
	req.ID = 1
	req.Status = "pending"
	req.CreatedAt = time.Now().UTC()
	return req, nil
}

// mockHandlerFly is a mock implementation of the Fly client for handler tests
type mockHandlerFly struct {
	createMachineErr  error
	stopMachineErr    error
	destroyMachineErr error
	machineID         string
}

func newMockHandlerFly() *mockHandlerFly {
	return &mockHandlerFly{
		machineID: "fly_machine_test_123",
	}
}

func (m *mockHandlerFly) CreateMachine(ctx context.Context, config *fly.MachineConfig) (*fly.Machine, error) {
	if m.createMachineErr != nil {
		return nil, m.createMachineErr
	}

	return &fly.Machine{
		ID:        m.machineID,
		State:     "created",
		Region:    "lhr",
		Config:    config,
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func (m *mockHandlerFly) StopMachine(ctx context.Context, machineID string) error {
	return m.stopMachineErr
}

func (m *mockHandlerFly) DestroyMachine(ctx context.Context, machineID string) error {
	return m.destroyMachineErr
}

func TestGenerateSessionID(t *testing.T) {
	// Test that session IDs have correct format
	for i := 0; i < 10; i++ {
		id := generateSessionID()
		if len(id) != 17 { // "sess_" + 12 hex chars
			t.Errorf("Expected session ID length 17, got %d", len(id))
		}
		if id[:5] != "sess_" {
			t.Errorf("Expected session ID to start with 'sess_', got '%s'", id[:5])
		}
	}

	// Test uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateSessionID()
		if ids[id] {
			t.Errorf("Generated duplicate session ID: %s", id)
		}
		ids[id] = true
	}
}

func TestBuildMachineConfig(t *testing.T) {
	tests := []struct {
		name     string
		request  *CreateSessionRequest
		validate func(*testing.T, *fly.MachineConfig)
	}{
		{
			name: "basic config",
			request: &CreateSessionRequest{
				Image:   "ubuntu:22.04",
				Command: []string{"bash", "-c", "echo hello"},
				Env: map[string]string{
					"FOO": "bar",
				},
			},
			validate: func(t *testing.T, config *fly.MachineConfig) {
				if config.Image != "ubuntu:22.04" {
					t.Errorf("Expected image 'ubuntu:22.04', got '%s'", config.Image)
				}
				if len(config.Cmd) != 3 {
					t.Errorf("Expected 3 command args, got %d", len(config.Cmd))
				}
				if config.Env["FOO"] != "bar" {
					t.Errorf("Expected env FOO='bar', got '%s'", config.Env["FOO"])
				}
			},
		},
		{
			name: "with resources",
			request: &CreateSessionRequest{
				Image: "ubuntu:22.04",
				Resources: &Resources{
					CPUMillis: 2500,
					MemoryMB:  1024,
				},
			},
			validate: func(t *testing.T, config *fly.MachineConfig) {
				if config.Guest == nil {
					t.Fatal("Expected guest config to be set")
				}
				if config.Guest.CPUs != 3 { // 2500ms rounds up to 3 CPUs
					t.Errorf("Expected 3 CPUs, got %d", config.Guest.CPUs)
				}
				if config.Guest.MemoryMB != 1024 {
					t.Errorf("Expected 1024 MB memory, got %d", config.Guest.MemoryMB)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildMachineConfig(tt.request)
			tt.validate(t, config)
		})
	}
}

func TestCreateSession_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	mockFly := newMockHandlerFly()
	handlers := NewHandlers(mockDB, mockFly)

	apiKeyID := uuid.New()

	request := CreateSessionRequest{
		Image:   "ubuntu:22.04",
		Command: []string{"bash", "-c", "echo hello"},
		Env: map[string]string{
			"FOO": "bar",
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader(body))
	req = req.WithContext(WithAPIKeyID(req.Context(), apiKeyID))

	w := httptest.NewRecorder()
	handlers.CreateSession(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response CreateSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ID == "" {
		t.Error("Expected session ID in response")
	}
	if response.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", response.Status)
	}
}

func TestGetSession_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	mockFly := newMockHandlerFly()
	handlers := NewHandlers(mockDB, mockFly)

	apiKeyID := uuid.New()
	machineID := "fly_machine_123"

	existingSession := &db.Session{
		ID:           "sess_test_123",
		APIKeyID:     apiKeyID,
		FlyMachineID: &machineID,
		Image:        "ubuntu:22.04",
		Status:       "running",
		CreatedAt:    time.Now().UTC(),
	}
	mockDB.sessions[existingSession.ID] = existingSession

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/sess_test_123", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sess_test_123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(WithAPIKeyID(req.Context(), apiKeyID))

	w := httptest.NewRecorder()
	handlers.GetSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response SessionResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.ID != "sess_test_123" {
		t.Errorf("Expected session ID 'sess_test_123', got '%s'", response.ID)
	}
}
