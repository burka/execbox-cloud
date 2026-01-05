//go:build integration

package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func getTestDB(t *testing.T) *Client {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	ctx := context.Background()
	client, err := New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

func createTestAPIKey(t *testing.T, client *Client, ctx context.Context) *APIKey {
	apiKeyID := uuid.New()
	keyStr := fmt.Sprintf("eb_test_%s", apiKeyID.String()[:8])

	query := `
		INSERT INTO api_keys (id, key, tier, rate_limit_rps, created_at)
		VALUES ($1, $2, 'free', 10, NOW())
	`
	_, err := client.pool.Exec(ctx, query, apiKeyID, keyStr)
	if err != nil {
		t.Fatalf("failed to create test API key: %v", err)
	}

	return &APIKey{
		ID:           apiKeyID,
		Key:          keyStr,
		Tier:         "free",
		RateLimitRPS: 10,
	}
}

func cleanupTestData(t *testing.T, client *Client, ctx context.Context, apiKeyID uuid.UUID) {
	// Clean up sessions first (due to foreign key)
	_, err := client.pool.Exec(ctx, "DELETE FROM sessions WHERE api_key_id = $1", apiKeyID)
	if err != nil {
		t.Logf("warning: failed to cleanup test sessions: %v", err)
	}

	// Clean up API key
	_, err = client.pool.Exec(ctx, "DELETE FROM api_keys WHERE id = $1", apiKeyID)
	if err != nil {
		t.Logf("warning: failed to cleanup test API key: %v", err)
	}
}

func TestGetAPIKeyByKey(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	got, err := client.GetAPIKeyByKey(ctx, apiKey.Key)
	if err != nil {
		t.Fatalf("GetAPIKeyByKey failed: %v", err)
	}

	if got.ID != apiKey.ID {
		t.Errorf("got ID %s, want %s", got.ID, apiKey.ID)
	}
	if got.Key != apiKey.Key {
		t.Errorf("got Key %s, want %s", got.Key, apiKey.Key)
	}
	if got.Tier != "free" {
		t.Errorf("got Tier %s, want free", got.Tier)
	}
}

func TestGetAPIKeyByKeyNotFound(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	_, err := client.GetAPIKeyByKey(ctx, "eb_nonexistent")
	if err == nil {
		t.Error("expected error for non-existent key, got nil")
	}
}

func TestUpdateAPIKeyLastUsed(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	// Update last used
	err := client.UpdateAPIKeyLastUsed(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed failed: %v", err)
	}

	// Verify it was updated
	got, err := client.GetAPIKeyByKey(ctx, apiKey.Key)
	if err != nil {
		t.Fatalf("GetAPIKeyByKey failed: %v", err)
	}

	if got.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after update")
	}
}

func TestCreateAndGetSession(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	session := &Session{
		ID:        "sess_test123",
		APIKeyID:  apiKey.ID,
		Image:     "python:3.12",
		Status:    "pending",
		Command:   []string{"python", "-c", "print('hello')"},
		CreatedAt: time.Now().UTC(),
	}

	err := client.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	got, err := client.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if got.ID != session.ID {
		t.Errorf("got ID %s, want %s", got.ID, session.ID)
	}
	if got.Image != session.Image {
		t.Errorf("got Image %s, want %s", got.Image, session.Image)
	}
	if got.Status != session.Status {
		t.Errorf("got Status %s, want %s", got.Status, session.Status)
	}
	if len(got.Command) != len(session.Command) {
		t.Errorf("got Command length %d, want %d", len(got.Command), len(session.Command))
	}
}

func TestGetSessionNotFound(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	_, err := client.GetSession(ctx, "sess_nonexistent")
	if err == nil {
		t.Error("expected error for non-existent session, got nil")
	}
}

func TestCreateSessionWithComplexData(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	session := &Session{
		ID:       "sess_complex",
		APIKeyID: apiKey.ID,
		Image:    "node:20",
		Status:   "pending",
		Command:  []string{"node", "-e", "console.log('test')"},
		Env: map[string]string{
			"NODE_ENV": "test",
			"DEBUG":    "true",
		},
		Ports: []Port{
			{Container: 3000, Host: 3000, Protocol: "tcp", URL: "http://localhost:3000"},
			{Container: 8080, Host: 8080, Protocol: "http", URL: "http://localhost:8080"},
		},
		CreatedAt: time.Now().UTC(),
	}

	err := client.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	got, err := client.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if len(got.Env) != 2 {
		t.Errorf("got %d env vars, want 2", len(got.Env))
	}
	if got.Env["NODE_ENV"] != "test" {
		t.Errorf("got NODE_ENV=%s, want test", got.Env["NODE_ENV"])
	}
	if len(got.Ports) != 2 {
		t.Errorf("got %d ports, want 2", len(got.Ports))
	}
}

func TestUpdateSession(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	session := &Session{
		ID:        "sess_update_test",
		APIKeyID:  apiKey.ID,
		Image:     "node:20",
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	if err := client.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update status
	newStatus := "running"
	now := time.Now().UTC()
	update := &SessionUpdate{
		Status:    &newStatus,
		StartedAt: &now,
	}

	if err := client.UpdateSession(ctx, session.ID, update); err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	got, _ := client.GetSession(ctx, session.ID)
	if got.Status != "running" {
		t.Errorf("got status %s, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
}

func TestUpdateSessionMultipleFields(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	session := &Session{
		ID:        "sess_multi_update",
		APIKeyID:  apiKey.ID,
		Image:     "alpine",
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	if err := client.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update multiple fields
	newStatus := "stopped"
	exitCode := 0
	flyMachineID := "fly_machine_123"
	flyAppID := "fly_app_123"
	now := time.Now().UTC()

	update := &SessionUpdate{
		Status:       &newStatus,
		ExitCode:     &exitCode,
		FlyMachineID: &flyMachineID,
		FlyAppID:     &flyAppID,
		EndedAt:      &now,
	}

	if err := client.UpdateSession(ctx, session.ID, update); err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	got, _ := client.GetSession(ctx, session.ID)
	if got.Status != "stopped" {
		t.Errorf("got status %s, want stopped", got.Status)
	}
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Error("ExitCode should be set to 0")
	}
	if got.FlyMachineID == nil || *got.FlyMachineID != "fly_machine_123" {
		t.Error("FlyMachineID should be set")
	}
}

func TestUpdateSessionNoFields(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	update := &SessionUpdate{}

	err := client.UpdateSession(ctx, "sess_any", update)
	if err == nil {
		t.Error("expected error when updating with no fields, got nil")
	}
}

func TestListSessions(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := &Session{
			ID:        fmt.Sprintf("sess_list_%d", i),
			APIKeyID:  apiKey.ID,
			Image:     "alpine",
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}
		if err := client.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	sessions, err := client.ListSessions(ctx, apiKey.ID, nil)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) < 3 {
		t.Errorf("got %d sessions, want at least 3", len(sessions))
	}
}

func TestListSessionsWithStatusFilter(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	// Create sessions with different statuses
	statuses := []string{"pending", "running", "stopped"}
	for i, status := range statuses {
		session := &Session{
			ID:        fmt.Sprintf("sess_filter_%d", i),
			APIKeyID:  apiKey.ID,
			Image:     "alpine",
			Status:    status,
			CreatedAt: time.Now().UTC(),
		}
		if err := client.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	// List only running sessions
	runningStatus := "running"
	sessions, err := client.ListSessions(ctx, apiKey.ID, &runningStatus)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) < 1 {
		t.Error("expected at least 1 running session")
	}
	for _, s := range sessions {
		if s.Status != "running" {
			t.Errorf("got status %s in filtered list, want running", s.Status)
		}
	}
}

func TestDeleteSession(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	session := &Session{
		ID:        "sess_delete_test",
		APIKeyID:  apiKey.ID,
		Image:     "alpine",
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	if err := client.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Delete the session
	if err := client.DeleteSession(ctx, session.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify it's gone
	_, err := client.GetSession(ctx, session.ID)
	if err == nil {
		t.Error("expected error when getting deleted session, got nil")
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	err := client.DeleteSession(ctx, "sess_nonexistent")
	if err == nil {
		t.Error("expected error when deleting non-existent session, got nil")
	}
}

func TestGetActiveSessionCount(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	// Create sessions with different statuses
	statuses := []string{"pending", "running", "stopped", "failed"}
	for i, status := range statuses {
		session := &Session{
			ID:        fmt.Sprintf("sess_count_%d", i),
			APIKeyID:  apiKey.ID,
			Image:     "alpine",
			Status:    status,
			CreatedAt: time.Now().UTC(),
		}
		if err := client.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	count, err := client.GetActiveSessionCount(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("GetActiveSessionCount failed: %v", err)
	}

	// Should count pending and running, not stopped or failed
	if count != 2 {
		t.Errorf("got count %d, want 2 (pending + running)", count)
	}
}

func TestGetDailySessionCount(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	// Create sessions
	for i := 0; i < 5; i++ {
		session := &Session{
			ID:        fmt.Sprintf("sess_daily_%d", i),
			APIKeyID:  apiKey.ID,
			Image:     "alpine",
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}
		if err := client.CreateSession(ctx, session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	count, err := client.GetDailySessionCount(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("GetDailySessionCount failed: %v", err)
	}

	if count < 5 {
		t.Errorf("got count %d, want at least 5", count)
	}
}

func TestIncrementUsage(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	// Increment usage
	err := client.IncrementUsage(ctx, apiKey.ID, 1000)
	if err != nil {
		t.Fatalf("IncrementUsage failed: %v", err)
	}

	// Increment again to test ON CONFLICT
	err = client.IncrementUsage(ctx, apiKey.ID, 2000)
	if err != nil {
		t.Fatalf("IncrementUsage (second call) failed: %v", err)
	}

	// Verify the usage was recorded
	query := `
		SELECT executions, duration_ms
		FROM usage_metrics
		WHERE api_key_id = $1 AND date = $2
	`
	today := time.Now().UTC().Truncate(24 * time.Hour)

	var executions int
	var durationMs int64
	err = client.pool.QueryRow(ctx, query, apiKey.ID, today).Scan(&executions, &durationMs)
	if err != nil {
		t.Fatalf("failed to query usage metrics: %v", err)
	}

	if executions != 2 {
		t.Errorf("got executions %d, want 2", executions)
	}
	if durationMs != 3000 {
		t.Errorf("got duration_ms %d, want 3000", durationMs)
	}
}

func TestImageCacheOperations(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	hash := "test_hash_123"
	baseImage := "python:3.12"
	registryTag := "registry.fly.io/test:latest"

	// Initially should not exist
	_, found, err := client.GetImageCache(ctx, hash)
	if err != nil {
		t.Fatalf("GetImageCache failed: %v", err)
	}
	if found {
		t.Error("expected cache miss, got hit")
	}

	// Put into cache
	err = client.PutImageCache(ctx, hash, baseImage, registryTag)
	if err != nil {
		t.Fatalf("PutImageCache failed: %v", err)
	}

	// Should now be found
	gotTag, found, err := client.GetImageCache(ctx, hash)
	if err != nil {
		t.Fatalf("GetImageCache failed: %v", err)
	}
	if !found {
		t.Error("expected cache hit, got miss")
	}
	if gotTag != registryTag {
		t.Errorf("got registry tag %s, want %s", gotTag, registryTag)
	}

	// Touch the cache entry
	err = client.TouchImageCache(ctx, hash)
	if err != nil {
		t.Fatalf("TouchImageCache failed: %v", err)
	}

	// Cleanup
	_, err = client.pool.Exec(ctx, "DELETE FROM image_cache WHERE hash = $1", hash)
	if err != nil {
		t.Logf("warning: failed to cleanup image cache: %v", err)
	}
}

func TestPutImageCacheDuplicate(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	hash := "test_dup_hash"
	baseImage := "alpine"
	registryTag := "registry.fly.io/test:v1"

	// Put into cache
	err := client.PutImageCache(ctx, hash, baseImage, registryTag)
	if err != nil {
		t.Fatalf("PutImageCache failed: %v", err)
	}

	// Put again with different values (should be ignored due to ON CONFLICT DO NOTHING)
	err = client.PutImageCache(ctx, hash, "different:image", "different:tag")
	if err != nil {
		t.Fatalf("PutImageCache (duplicate) failed: %v", err)
	}

	// Should still have original value
	gotTag, found, err := client.GetImageCache(ctx, hash)
	if err != nil {
		t.Fatalf("GetImageCache failed: %v", err)
	}
	if !found {
		t.Error("expected cache hit, got miss")
	}
	if gotTag != registryTag {
		t.Errorf("got registry tag %s, want original %s", gotTag, registryTag)
	}

	// Cleanup
	_, err = client.pool.Exec(ctx, "DELETE FROM image_cache WHERE hash = $1", hash)
	if err != nil {
		t.Logf("warning: failed to cleanup image cache: %v", err)
	}
}

func TestCreateQuotaRequest(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	name := "Test User"
	requestedLimits := "1000 sessions/day"

	req := &QuotaRequest{
		APIKeyID:        &apiKey.ID,
		Email:           "test@example.com",
		Name:            &name,
		RequestedLimits: &requestedLimits,
	}

	created, err := client.CreateQuotaRequest(ctx, req)
	if err != nil {
		t.Fatalf("CreateQuotaRequest failed: %v", err)
	}

	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Status != "pending" {
		t.Errorf("got status %s, want pending", created.Status)
	}
	if created.Email != "test@example.com" {
		t.Errorf("got email %s, want test@example.com", created.Email)
	}

	// Cleanup
	_, err = client.pool.Exec(ctx, "DELETE FROM quota_requests WHERE id = $1", created.ID)
	if err != nil {
		t.Logf("warning: failed to cleanup quota request: %v", err)
	}
}

func TestCreateQuotaRequestWithAllFields(t *testing.T) {
	client := getTestDB(t)
	ctx := context.Background()

	apiKey := createTestAPIKey(t, client, ctx)
	defer cleanupTestData(t, client, ctx, apiKey.ID)

	name := "Test User"
	company := "Test Company"
	currentTier := "free"
	requestedLimits := "5000 sessions/day"
	budget := "$500/month"
	useCase := "Running production workloads"

	req := &QuotaRequest{
		APIKeyID:        &apiKey.ID,
		Email:           "test@example.com",
		Name:            &name,
		Company:         &company,
		CurrentTier:     &currentTier,
		RequestedLimits: &requestedLimits,
		Budget:          &budget,
		UseCase:         &useCase,
	}

	created, err := client.CreateQuotaRequest(ctx, req)
	if err != nil {
		t.Fatalf("CreateQuotaRequest failed: %v", err)
	}

	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Company == nil || *created.Company != "Test Company" {
		t.Error("Company should be set")
	}
	if created.Budget == nil || *created.Budget != "$500/month" {
		t.Error("Budget should be set")
	}

	// Cleanup
	_, err = client.pool.Exec(ctx, "DELETE FROM quota_requests WHERE id = $1", created.ID)
	if err != nil {
		t.Logf("warning: failed to cleanup quota request: %v", err)
	}
}

func stringPtr(s string) *string {
	return &s
}
