package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extendedMockHandlerDB extends the mockHandlerDB for account-specific tests
type extendedMockHandlerDB struct {
	*mockHandlerDB
	hourlyUsage    []db.HourlyAccountUsage
	dailyUsage     []db.UsageMetric
	accountLimits  *db.AccountLimits
	getLimitsError error
}

func newExtendedMockHandlerDB() *extendedMockHandlerDB {
	return &extendedMockHandlerDB{
		mockHandlerDB: newMockHandlerDB(),
	}
}

func (m *extendedMockHandlerDB) GetHourlyAccountUsage(ctx context.Context, accountID uuid.UUID, start, end time.Time) ([]db.HourlyAccountUsage, error) {
	return m.hourlyUsage, nil
}

func (m *extendedMockHandlerDB) GetDailyAccountUsage(ctx context.Context, accountID uuid.UUID, days int) ([]db.UsageMetric, error) {
	return m.dailyUsage, nil
}

func (m *extendedMockHandlerDB) GetAccountLimits(ctx context.Context, accountID uuid.UUID) (*db.AccountLimits, error) {
	if m.getLimitsError != nil {
		return nil, m.getLimitsError
	}
	if m.accountLimits != nil {
		return m.accountLimits, nil
	}
	return nil, nil // Simulate not found
}

func (m *extendedMockHandlerDB) UpsertAccountLimits(ctx context.Context, limits *db.AccountLimits) error {
	m.accountLimits = limits
	return nil
}

func TestAccountService_GetEnhancedUsage_Success(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()

	// Set up mock data
	apiKeyID := uuid.New()

	// Create test sessions to simulate daily and active counts
	for i := 0; i < 10; i++ {
		sessionID := fmt.Sprintf("sess_test_%d", i)
		status := "stopped"
		if i < 2 {
			status = "running" // 2 active sessions
		}
		mockDB.sessions[sessionID] = &db.Session{
			ID:        sessionID,
			APIKeyID:  apiKeyID,
			Status:    status,
			CreatedAt: time.Now().UTC(),
		}
	}

	// Mock hourly usage
	now := time.Now().UTC()
	hourlyData := []db.HourlyAccountUsage{
		{
			Hour:              now.Add(-1 * time.Hour),
			Executions:        5,
			CostEstimateCents: 25,
			Errors:            1,
		},
		{
			Hour:              now.Add(-2 * time.Hour),
			Executions:        3,
			CostEstimateCents: 15,
			Errors:            0,
		},
	}
	mockDB.hourlyUsage = hourlyData

	// Mock daily usage
	dailyData := []db.UsageMetric{
		{
			Date:       now.AddDate(0, 0, -1),
			Executions: 20,
			DurationMs: 120000, // 2 minutes
		},
		{
			Date:       now,
			Executions: 10,
			DurationMs: 60000, // 1 minute
		},
	}
	mockDB.dailyUsage = dailyData

	service := NewAccountService(mockDB)

	// Create context with API key
	ctx := WithAPIKeyID(context.Background(), apiKeyID)
	ctx = WithAPIKeyTier(ctx, "free")

	input := &GetEnhancedUsageInput{Days: 7}
	output, err := service.GetEnhancedUsage(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, 10, output.Body.SessionsToday)
	assert.Equal(t, 2, output.Body.ActiveSessions)
	assert.Equal(t, "free", output.Body.Tier)
	assert.Equal(t, apiKeyID.String(), output.Body.AccountID)
	assert.Len(t, output.Body.HourlyUsage, 2)
	assert.Len(t, output.Body.DailyHistory, 2)
	assert.Greater(t, output.Body.CostEstimateCents, int64(0))
	assert.Equal(t, 80, output.Body.AlertThreshold)

	// Verify hourly data
	assert.Equal(t, 5, output.Body.HourlyUsage[0].Executions)
	assert.Equal(t, int64(25), output.Body.HourlyUsage[0].CostCents)
	assert.Equal(t, 1, output.Body.HourlyUsage[0].Errors)

	// Verify daily data
	assert.Equal(t, 20, output.Body.DailyHistory[0].Executions)
	assert.Equal(t, int64(120000), output.Body.DailyHistory[0].DurationMs)
}

func TestAccountService_GetEnhancedUsage_Unauthorized(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	service := NewAccountService(mockDB)

	// Call without API key context
	ctx := context.Background()
	input := &GetEnhancedUsageInput{Days: 7}

	_, err := service.GetEnhancedUsage(ctx, input)

	// Should return 401 unauthorized error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestAccountService_GetAccountLimits_Success(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	apiKeyID := uuid.New()

	// Mock existing limits
	monthlyCost := int64(5000)
	billingEmail := "billing@example.com"
	limits := &db.AccountLimits{
		AccountID:                apiKeyID,
		DailyRequestsLimit:       100,
		ConcurrentRequestsLimit:  10,
		MonthlyCostLimitCents:    &monthlyCost,
		AlertThresholdPercentage: 75,
		BillingEmail:             &billingEmail,
		Timezone:                 "America/New_York",
	}
	mockDB.accountLimits = limits

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	input := &GetAccountLimitsInput{}
	output, err := service.GetAccountLimits(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, 100, output.Body.DailyRequestsLimit)
	assert.Equal(t, 10, output.Body.ConcurrentRequestsLimit)
	require.NotNil(t, output.Body.MonthlyCostLimitCents)
	assert.Equal(t, int64(5000), *output.Body.MonthlyCostLimitCents)
	assert.Equal(t, 75, output.Body.AlertThreshold)
	require.NotNil(t, output.Body.BillingEmail)
	assert.Equal(t, "billing@example.com", *output.Body.BillingEmail)
	assert.Equal(t, "America/New_York", output.Body.Timezone)
}

func TestAccountService_GetAccountLimits_NotFound_ReturnsDefaults(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	apiKeyID := uuid.New()

	// Mock "not found" error
	mockDB.getLimitsError = fmt.Errorf("account limits not found")
	mockDB.accountLimits = nil // Ensure no limits are set

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), apiKeyID)
	ctx = WithAPIKeyTier(ctx, "free")

	input := &GetAccountLimitsInput{}
	output, err := service.GetAccountLimits(ctx, input)

	require.NoError(t, err)

	// Should return default tier limits
	assert.Equal(t, 10, output.Body.DailyRequestsLimit)     // Free tier default
	assert.Equal(t, 5, output.Body.ConcurrentRequestsLimit) // Free tier default
	assert.Equal(t, 80, output.Body.AlertThreshold)         // Default
	assert.Equal(t, "UTC", output.Body.Timezone)            // Default
}

func TestAccountService_UpdateAccountLimits_Success(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	apiKeyID := uuid.New()

	// Set up existing limits (will be retrieved by GetAccountLimits)
	monthlyCost := int64(2000)
	billingEmail := "old@example.com"
	existingLimits := &db.AccountLimits{
		AccountID:                apiKeyID,
		DailyRequestsLimit:       50,
		ConcurrentRequestsLimit:  5,
		MonthlyCostLimitCents:    &monthlyCost,
		AlertThresholdPercentage: 70,
		BillingEmail:             &billingEmail,
		Timezone:                 "UTC",
	}
	mockDB.accountLimits = existingLimits

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	// Send partial update - only update specific fields
	newBillingEmail := "new@example.com"
	newAlertThreshold := 85
	input := &UpdateAccountLimitsInput{
		Body: UpdateAccountLimitsRequest{
			BillingEmail:   &newBillingEmail,
			AlertThreshold: &newAlertThreshold,
			// DailyRequestsLimit and other fields nil - should remain unchanged
		},
	}

	output, err := service.UpdateAccountLimits(ctx, input)

	require.NoError(t, err)

	// Verify only specified fields changed, others remain the same
	assert.Equal(t, 50, output.Body.DailyRequestsLimit)     // Unchanged
	assert.Equal(t, 5, output.Body.ConcurrentRequestsLimit) // Unchanged
	require.NotNil(t, output.Body.MonthlyCostLimitCents)
	assert.Equal(t, int64(2000), *output.Body.MonthlyCostLimitCents) // Unchanged
	assert.Equal(t, 85, output.Body.AlertThreshold)                  // Updated
	require.NotNil(t, output.Body.BillingEmail)
	assert.Equal(t, "new@example.com", *output.Body.BillingEmail) // Updated
	assert.Equal(t, "UTC", output.Body.Timezone)                  // Unchanged

	// Verify the mock was updated correctly
	assert.Equal(t, 85, mockDB.accountLimits.AlertThresholdPercentage)
	require.NotNil(t, mockDB.accountLimits.BillingEmail)
	assert.Equal(t, "new@example.com", *mockDB.accountLimits.BillingEmail)
	assert.Equal(t, 50, mockDB.accountLimits.DailyRequestsLimit) // Unchanged
}

func TestAccountService_ExportUsage_Success(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	apiKeyID := uuid.New()

	// Mock daily usage
	now := time.Now().UTC()
	dailyData := []db.UsageMetric{
		{
			Date:       now.AddDate(0, 0, -2),
			Executions: 15,
			DurationMs: 90000, // 1.5 minutes
		},
		{
			Date:       now.AddDate(0, 0, -1),
			Executions: 25,
			DurationMs: 150000, // 2.5 minutes
		},
		{
			Date:       now,
			Executions: 8,
			DurationMs: 48000, // 0.8 minutes
		},
	}
	mockDB.dailyUsage = dailyData

	// Mock hourly usage for error calculation
	hourlyData := []db.HourlyAccountUsage{
		{
			Hour:       now.AddDate(0, 0, -1),
			Executions: 20,
			Errors:     2,
		},
		{
			Hour:       now,
			Executions: 8,
			Errors:     1,
		},
	}
	mockDB.hourlyUsage = hourlyData

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), apiKeyID)

	input := &ExportUsageInput{Days: 3}
	output, err := service.ExportUsage(ctx, input)

	require.NoError(t, err)
	assert.Len(t, output.Body, 3)

	// Verify first day data
	assert.Equal(t, 15, output.Body[0].Executions)
	assert.Equal(t, int64(90000), output.Body[0].DurationMs)
	assert.Greater(t, output.Body[0].CostCents, int64(0))

	// Verify second day data (with errors)
	assert.Equal(t, 25, output.Body[1].Executions)
	assert.Equal(t, int64(150000), output.Body[1].DurationMs)
	assert.Equal(t, 2, output.Body[1].Errors) // From hourly data

	// Verify current day data (with errors)
	assert.Equal(t, 8, output.Body[2].Executions)
	assert.Equal(t, int64(48000), output.Body[2].DurationMs)
	assert.Equal(t, 1, output.Body[2].Errors) // From hourly data
}

func TestAccountService_ExportUsage_Unauthorized(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	service := NewAccountService(mockDB)

	// Call without API key context
	ctx := context.Background()
	input := &ExportUsageInput{Days: 7}

	_, err := service.ExportUsage(ctx, input)

	// Should return 401 unauthorized error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestAccountService_UpdateAccountLimits_Unauthorized(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	service := NewAccountService(mockDB)

	// Call without API key context
	ctx := context.Background()
	input := &UpdateAccountLimitsInput{
		Body: UpdateAccountLimitsRequest{
			DailyRequestsLimit: func() *int { v := 100; return &v }(),
		},
	}

	_, err := service.UpdateAccountLimits(ctx, input)

	// Should return 401 unauthorized error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestAccountService_GetAccountLimits_Unauthorized(t *testing.T) {
	mockDB := newExtendedMockHandlerDB()
	service := NewAccountService(mockDB)

	// Call without API key context
	ctx := context.Background()
	input := &GetAccountLimitsInput{}

	_, err := service.GetAccountLimits(ctx, input)

	// Should return 401 unauthorized error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}
