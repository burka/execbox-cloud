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

func (m *mockDB) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*db.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, apiKey := range m.apiKeys {
		if apiKey.ID == id {
			// Return a copy to avoid race conditions
			keyCopy := *apiKey
			return &keyCopy, nil
		}
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockDB) CreateAPIKey(ctx context.Context, email string, name *string) (*db.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("sk_test_%s", randHex(32))
	apiKey := &db.APIKey{
		ID:           uuid.New(),
		Key:          key,
		Email:        &email,
		Tier:         "free",
		RateLimitRPS: 10,
		CreatedAt:    time.Now().UTC(),
	}
	m.apiKeys[key] = apiKey
	return apiKey, nil
}

// Account-level usage query stubs

func (m *mockDB) GetAccountLimits(ctx context.Context, accountID uuid.UUID) (*db.AccountLimits, error) {
	return nil, fmt.Errorf("account limits not found")
}

func (m *mockDB) UpsertAccountLimits(ctx context.Context, limits *db.AccountLimits) error {
	return nil
}

func (m *mockDB) GetHourlyAccountUsage(ctx context.Context, accountID uuid.UUID, start, end time.Time) ([]db.HourlyAccountUsage, error) {
	return nil, nil
}

func (m *mockDB) GetDailyAccountUsage(ctx context.Context, accountID uuid.UUID, days int) ([]db.UsageMetric, error) {
	return nil, nil
}

func (m *mockDB) GetAccountCostTracking(ctx context.Context, accountID uuid.UUID, periodStart time.Time) ([]db.AccountCostTracking, error) {
	return nil, nil
}

// Multi-key management stubs

func (m *mockDB) GetAPIKeysByAccount(ctx context.Context, accountID uuid.UUID) ([]db.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []db.APIKey
	for _, apiKey := range m.apiKeys {
		if apiKey.AccountID == accountID {
			keyCopy := *apiKey
			keys = append(keys, keyCopy)
		}
	}
	return keys, nil
}

func (m *mockDB) CreateAPIKeyForAccount(ctx context.Context, accountID uuid.UUID, name, description string, parentKeyID uuid.UUID) (*db.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("sk_test_%s", randHex(32))
	apiKey := &db.APIKey{
		ID:          uuid.New(),
		Key:         key,
		Tier:        "free",
		RateLimitRPS: 10,
		CreatedAt:   time.Now().UTC(),
		IsActive:    true,
		AccountID:   accountID,
		ParentKeyID: &parentKeyID,
		Name:        &name,
		Description: &description,
	}
	m.apiKeys[key] = apiKey
	return apiKey, nil
}

func (m *mockDB) UpdateAPIKey(ctx context.Context, keyID uuid.UUID, update *db.APIKeyUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID {
			if update.Name != nil {
				apiKey.Name = update.Name
			}
			if update.Description != nil {
				apiKey.Description = update.Description
			}
			if update.IsActive != nil {
				apiKey.IsActive = *update.IsActive
			}
			if update.ExpiresAt != nil {
				apiKey.ExpiresAt = update.ExpiresAt
			}
			if update.CustomDailyLimit != nil {
				apiKey.CustomDailyLimit = update.CustomDailyLimit
			}
			if update.CustomConcurrentLimit != nil {
				apiKey.CustomConcurrentLimit = update.CustomConcurrentLimit
			}
			if update.LastUpdatedBy != nil {
				apiKey.LastUpdatedBy = update.LastUpdatedBy
			}
			return nil
		}
	}
	return fmt.Errorf("API key not found")
}

func (m *mockDB) DeactivateAPIKey(ctx context.Context, keyID uuid.UUID, performedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID {
			apiKey.IsActive = false
			apiKey.LastUpdatedBy = &performedBy
			return nil
		}
	}
	return fmt.Errorf("API key not found")
}

func (m *mockDB) RotateAPIKey(ctx context.Context, keyID uuid.UUID, performedBy string) (*db.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for oldKey, apiKey := range m.apiKeys {
		if apiKey.ID == keyID {
			// Remove old key mapping
			delete(m.apiKeys, oldKey)

			// Generate new key
			newKey := fmt.Sprintf("sk_test_%s", randHex(32))
			apiKey.Key = newKey
			apiKey.LastUpdatedBy = &performedBy

			// Store with new key
			m.apiKeys[newKey] = apiKey

			keyCopy := *apiKey
			return &keyCopy, nil
		}
	}
	return nil, fmt.Errorf("API key not found")
}

func (m *mockDB) IsPrimaryKey(ctx context.Context, keyID uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, apiKey := range m.apiKeys {
		if apiKey.ID == keyID {
			// Primary keys have account_id == their own ID, or no parent key
			return apiKey.AccountID == apiKey.ID || apiKey.ParentKeyID == nil, nil
		}
	}
	return false, fmt.Errorf("API key not found")
}
