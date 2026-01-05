package db

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSessionDefaults(t *testing.T) {
	sess := Session{
		ID:       "sess_test",
		APIKeyID: uuid.New(),
		Image:    "alpine",
		Status:   "pending",
	}

	if sess.ID == "" {
		t.Error("ID should not be empty")
	}
	if sess.Status != "pending" {
		t.Errorf("Status = %s, want pending", sess.Status)
	}
	if sess.Image != "alpine" {
		t.Errorf("Image = %s, want alpine", sess.Image)
	}
}

func TestAPIKeyFields(t *testing.T) {
	now := time.Now()
	key := APIKey{
		ID:        uuid.New(),
		Key:       "eb_test123",
		Tier:      "free",
		CreatedAt: now,
	}

	if key.Tier != "free" {
		t.Errorf("Tier = %s, want free", key.Tier)
	}
	if key.Key != "eb_test123" {
		t.Errorf("Key = %s, want eb_test123", key.Key)
	}
}

func TestPortStructure(t *testing.T) {
	port := Port{
		Container: 8080,
		Host:      8080,
		Protocol:  "tcp",
		URL:       "http://localhost:8080",
	}

	if port.Container != 8080 {
		t.Errorf("Container port = %d, want 8080", port.Container)
	}
	if port.Protocol != "tcp" {
		t.Errorf("Protocol = %s, want tcp", port.Protocol)
	}
}

func TestUsageMetricStructure(t *testing.T) {
	apiKeyID := uuid.New()
	metric := UsageMetric{
		ID:         1,
		APIKeyID:   apiKeyID,
		Date:       time.Now().UTC().Truncate(24 * time.Hour),
		Executions: 100,
		DurationMs: 50000,
	}

	if metric.Executions != 100 {
		t.Errorf("Executions = %d, want 100", metric.Executions)
	}
	if metric.DurationMs != 50000 {
		t.Errorf("DurationMs = %d, want 50000", metric.DurationMs)
	}
}

func TestSessionUpdateOptionalFields(t *testing.T) {
	status := "running"
	exitCode := 0
	now := time.Now()
	flyMachineID := "fly_machine_123"
	flyAppID := "fly_app_123"

	update := SessionUpdate{
		Status:       &status,
		ExitCode:     &exitCode,
		StartedAt:    &now,
		EndedAt:      &now,
		FlyMachineID: &flyMachineID,
		FlyAppID:     &flyAppID,
	}

	if update.Status == nil || *update.Status != "running" {
		t.Error("Status should be set to running")
	}
	if update.ExitCode == nil || *update.ExitCode != 0 {
		t.Error("ExitCode should be set to 0")
	}
	if update.FlyMachineID == nil || *update.FlyMachineID != "fly_machine_123" {
		t.Error("FlyMachineID should be set")
	}
}

func TestQuotaRequestStructure(t *testing.T) {
	apiKeyID := uuid.New()
	name := "Test User"
	company := "Test Company"
	currentTier := "free"
	requestedLimits := "1000 sessions/day"
	budget := "$100/month"
	useCase := "Testing purposes"

	req := QuotaRequest{
		ID:              1,
		APIKeyID:        &apiKeyID,
		Email:           "test@example.com",
		Name:            &name,
		Company:         &company,
		CurrentTier:     &currentTier,
		RequestedLimits: &requestedLimits,
		Budget:          &budget,
		UseCase:         &useCase,
		Status:          "pending",
		CreatedAt:       time.Now(),
	}

	if req.Email != "test@example.com" {
		t.Errorf("Email = %s, want test@example.com", req.Email)
	}
	if req.Status != "pending" {
		t.Errorf("Status = %s, want pending", req.Status)
	}
	if req.Name == nil || *req.Name != "Test User" {
		t.Error("Name should be set to Test User")
	}
}

func TestSessionWithComplexData(t *testing.T) {
	sess := Session{
		ID:       "sess_complex",
		APIKeyID: uuid.New(),
		Image:    "python:3.12",
		Command:  []string{"python", "-c", "print('hello')"},
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
		Ports: []Port{
			{Container: 8080, Host: 8080, Protocol: "tcp"},
			{Container: 3000, Host: 3000, Protocol: "http"},
		},
		Status: "pending",
	}

	if len(sess.Command) != 3 {
		t.Errorf("Command length = %d, want 3", len(sess.Command))
	}
	if len(sess.Env) != 2 {
		t.Errorf("Env length = %d, want 2", len(sess.Env))
	}
	if len(sess.Ports) != 2 {
		t.Errorf("Ports length = %d, want 2", len(sess.Ports))
	}
	if sess.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO] = %s, want bar", sess.Env["FOO"])
	}
}
