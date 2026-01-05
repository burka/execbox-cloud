package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/google/uuid"
)

// mockDB implements a test double for db.Client
type mockDB struct {
	mu            sync.RWMutex
	apiKeys       map[string]*db.APIKey
	lastUsedCalls map[uuid.UUID]int
	sessions      map[string]*db.Session
}

func newMockDB() *mockDB {
	return &mockDB{
		apiKeys:       make(map[string]*db.APIKey),
		lastUsedCalls: make(map[uuid.UUID]int),
		sessions:      make(map[string]*db.Session),
	}
}

func (m *mockDB) GetAPIKeyByKey(ctx context.Context, key string) (*db.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apiKey, ok := m.apiKeys[key]
	if !ok {
		return nil, fmt.Errorf("API key not found")
	}
	// Return a copy to avoid race conditions
	keyCopy := *apiKey
	return &keyCopy, nil
}

func (m *mockDB) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastUsedCalls[id]++
	return nil
}

func (m *mockDB) GetSession(ctx context.Context, id string) (*db.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	// Return a copy to avoid race conditions
	sessCopy := *session
	return &sessCopy, nil
}

func (m *mockDB) CreateSession(ctx context.Context, sess *db.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[sess.ID] = sess
	return nil
}

func (m *mockDB) UpdateSession(ctx context.Context, id string, update *db.SessionUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found")
	}

	// Apply updates
	if update.Status != nil {
		session.Status = *update.Status
	}
	if update.ExitCode != nil {
		session.ExitCode = update.ExitCode
	}
	if update.StartedAt != nil {
		session.StartedAt = update.StartedAt
	}
	if update.EndedAt != nil {
		session.EndedAt = update.EndedAt
	}
	if update.FlyMachineID != nil {
		session.FlyMachineID = update.FlyMachineID
	}
	if update.FlyAppID != nil {
		session.FlyAppID = update.FlyAppID
	}
	if update.Ports != nil {
		session.Ports = update.Ports
	}

	return nil
}

func (m *mockDB) ListSessions(ctx context.Context, apiKeyID uuid.UUID, status *string) ([]db.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []db.Session
	for _, session := range m.sessions {
		if session.APIKeyID == apiKeyID {
			if status == nil || session.Status == *status {
				sessCopy := *session
				sessions = append(sessions, sessCopy)
			}
		}
	}

	return sessions, nil
}

func (m *mockDB) DeleteSession(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found")
	}

	delete(m.sessions, id)
	return nil
}

func (m *mockDB) GetActiveSessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, session := range m.sessions {
		if session.APIKeyID == apiKeyID && (session.Status == "running" || session.Status == "pending") {
			count++
		}
	}
	return count, nil
}

func (m *mockDB) GetDailySessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, session := range m.sessions {
		if session.APIKeyID == apiKeyID {
			count++
		}
	}
	return count, nil
}

func (m *mockDB) CreateQuotaRequest(ctx context.Context, req *db.QuotaRequest) (*db.QuotaRequest, error) {
	req.ID = 1
	req.Status = "pending"
	req.CreatedAt = time.Now().UTC()
	return req, nil
}
