package api

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
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

func (m *mockHandlerDB) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*db.APIKey, error) {
	for _, apiKey := range m.apiKeysByString {
		if apiKey.ID == id {
			return apiKey, nil
		}
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockHandlerDB) CreateAPIKey(ctx context.Context, email string, name *string) (*db.APIKey, error) {
	key := fmt.Sprintf("sk_test_%s", randHex(32))
	apiKey := &db.APIKey{
		ID:           uuid.New(),
		Key:          key,
		Email:        &email,
		Tier:         "free",
		RateLimitRPS: 10,
		CreatedAt:    time.Now().UTC(),
	}
	m.apiKeysByString[key] = apiKey
	return apiKey, nil
}

func (m *mockHandlerDB) GetAccountLimits(ctx context.Context, accountID uuid.UUID) (*db.AccountLimits, error) {
	return nil, nil
}

func (m *mockHandlerDB) UpsertAccountLimits(ctx context.Context, limits *db.AccountLimits) error {
	return nil
}

func (m *mockHandlerDB) GetHourlyAccountUsage(ctx context.Context, accountID uuid.UUID, start, end time.Time) ([]db.HourlyAccountUsage, error) {
	return nil, nil
}

func (m *mockHandlerDB) GetDailyAccountUsage(ctx context.Context, accountID uuid.UUID, days int) ([]db.UsageMetric, error) {
	return nil, nil
}

func (m *mockHandlerDB) GetAccountCostTracking(ctx context.Context, accountID uuid.UUID, periodStart time.Time) ([]db.AccountCostTracking, error) {
	return nil, nil
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

// Tests for the new huma-based services

func TestSessionService_CreateSession_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	mockBackend := &mockBackendHandler{}
	sessionSvc := NewSessionService(mockDB, mockBackend)

	apiKeyID := uuid.New()

	input := &CreateSessionInput{
		Body: CreateSessionRequest{
			Image:   "ubuntu:22.04",
			Command: []string{"bash", "-c", "echo hello"},
			Env: map[string]string{
				"FOO": "bar",
			},
		},
	}

	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	output, err := sessionSvc.CreateSession(ctx, input)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if output.Body.ID == "" {
		t.Error("Expected session ID in response")
	}
	if output.Body.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", output.Body.Status)
	}
}

func TestSessionService_GetSession_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	mockBackend := &mockBackendHandler{}
	sessionSvc := NewSessionService(mockDB, mockBackend)

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

	input := &GetSessionInput{
		ID: "sess_test_123",
	}

	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	output, err := sessionSvc.GetSession(ctx, input)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if output.Body.ID != "sess_test_123" {
		t.Errorf("Expected session ID 'sess_test_123', got '%s'", output.Body.ID)
	}
}

func TestAccountService_GetAccount_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	accountSvc := NewAccountService(mockDB)

	apiKeyID := uuid.New()
	email := "test@example.com"
	createdAt := time.Now().UTC()

	// Create test API key
	testKey := &db.APIKey{
		ID:           apiKeyID,
		Key:          "sk_test_12345678901234567890123456789012",
		Email:        &email,
		Tier:         "free",
		RateLimitRPS: 10,
		CreatedAt:    createdAt,
	}
	mockDB.apiKeysByString[testKey.Key] = testKey

	input := &GetAccountInput{}
	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	output, err := accountSvc.GetAccount(ctx, input)
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}

	if output.Body.Tier != "free" {
		t.Errorf("Expected tier 'free', got '%s'", output.Body.Tier)
	}
	if output.Body.Email == nil || *output.Body.Email != email {
		t.Errorf("Expected email '%s', got '%v'", email, output.Body.Email)
	}
	if output.Body.APIKeyID != apiKeyID.String() {
		t.Errorf("Expected API key ID '%s', got '%s'", apiKeyID.String(), output.Body.APIKeyID)
	}
	if output.Body.APIKeyPreview != "sk_test...9012" {
		t.Errorf("Expected masked key 'sk_test...9012', got '%s'", output.Body.APIKeyPreview)
	}
}

func TestAccountService_GetUsage_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	accountSvc := NewAccountService(mockDB)

	apiKeyID := uuid.New()

	// Create some test sessions
	for i := 0; i < 3; i++ {
		sessionID := fmt.Sprintf("sess_test_%d", i)
		mockDB.sessions[sessionID] = &db.Session{
			ID:        sessionID,
			APIKeyID:  apiKeyID,
			Status:    "running",
			CreatedAt: time.Now().UTC(),
		}
	}

	input := &GetUsageInput{}
	ctx := WithAPIKeyID(context.Background(), apiKeyID)
	ctx = WithAPIKeyTier(ctx, "free")

	output, err := accountSvc.GetUsage(ctx, input)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}

	if output.Body.SessionsToday != 3 {
		t.Errorf("Expected 3 sessions today, got %d", output.Body.SessionsToday)
	}
	if output.Body.ActiveSessions != 3 {
		t.Errorf("Expected 3 active sessions, got %d", output.Body.ActiveSessions)
	}
	if output.Body.Tier != "free" {
		t.Errorf("Expected tier 'free', got '%s'", output.Body.Tier)
	}
	if output.Body.DailyLimit != 10 {
		t.Errorf("Expected daily limit 10, got %d", output.Body.DailyLimit)
	}
	if output.Body.QuotaRemaining != 7 {
		t.Errorf("Expected quota remaining 7, got %d", output.Body.QuotaRemaining)
	}
}

func TestAccountService_JoinWaitlist_Success(t *testing.T) {
	mockDB := newMockHandlerDB()
	accountSvc := NewAccountService(mockDB)

	input := &JoinWaitlistInput{
		Body: WaitlistRequest{
			Email: "test@example.com",
		},
	}

	output, err := accountSvc.JoinWaitlist(context.Background(), input)
	if err != nil {
		t.Fatalf("JoinWaitlist failed: %v", err)
	}

	if output.Body.ID == "" {
		t.Error("Expected API key ID in response")
	}
	if output.Body.Key == "" {
		t.Error("Expected API key in response")
	}
	if output.Body.Tier != "free" {
		t.Errorf("Expected tier 'free', got '%s'", output.Body.Tier)
	}
	if output.Body.Message == "" {
		t.Error("Expected message in response")
	}
}

func TestAccountService_JoinWaitlist_MissingEmail(t *testing.T) {
	mockDB := newMockHandlerDB()
	accountSvc := NewAccountService(mockDB)

	input := &JoinWaitlistInput{
		Body: WaitlistRequest{
			Email: "",
		},
	}

	_, err := accountSvc.JoinWaitlist(context.Background(), input)
	if err == nil {
		t.Fatal("Expected error for missing email")
	}
}

// mockBackendHandler is a mock implementation of the Backend interface for handler tests
type mockBackendHandler struct {
	createErr  error
	stopErr    error
	destroyErr error
	backendID  string
}

func (m *mockBackendHandler) Name() string {
	return "mock"
}

func (m *mockBackendHandler) CreateSession(ctx context.Context, config *CreateSessionConfig) (*Session, *SessionNetwork, error) {
	if m.createErr != nil {
		return nil, nil, m.createErr
	}

	backendID := m.backendID
	if backendID == "" {
		backendID = "mock_backend_123"
	}

	return &Session{
		BackendID: backendID,
		Status:    "running",
	}, nil, nil
}

func (m *mockBackendHandler) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return &Session{
		ID:        sessionID,
		BackendID: "mock_backend_123",
		Status:    "running",
	}, nil
}

func (m *mockBackendHandler) StopSession(ctx context.Context, backendID string) error {
	return m.stopErr
}

func (m *mockBackendHandler) DestroySession(ctx context.Context, backendID string) error {
	return m.destroyErr
}

func (m *mockBackendHandler) Attach(ctx context.Context, sessionID string) (stdin io.WriteCloser, stdout io.Reader, stderr io.Reader, wait func() int, err error) {
	return nil, nil, nil, nil, fmt.Errorf("attach not implemented in mock backend")
}
