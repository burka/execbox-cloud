package api

import (
	"context"
	"strings"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/danielgtaylor/huma/v2"
)

// AccountService handles account and API key operations.
type AccountService struct {
	db DBClient
}

// NewAccountService creates a new AccountService.
func NewAccountService(db DBClient) *AccountService {
	return &AccountService{db: db}
}

// GetAccount handles GET /v1/account
// Returns account information for the authenticated API key.
func (a *AccountService) GetAccount(ctx context.Context, input *GetAccountInput) (*GetAccountOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get full API key details from database
	apiKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get API key", err)
	}

	// Build response
	response := AccountResponse{
		Tier:          apiKey.Tier,
		Email:         apiKey.Email,
		APIKeyID:      apiKey.ID.String(),
		APIKeyPreview: maskAPIKey(apiKey.Key),
		CreatedAt:     apiKey.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if apiKey.TierExpiresAt != nil {
		expiresAt := apiKey.TierExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		response.TierExpiresAt = &expiresAt
	}

	return &GetAccountOutput{
		Body: response,
	}, nil
}

// GetUsage handles GET /v1/account/usage
// Returns usage statistics for the authenticated API key.
func (a *AccountService) GetUsage(ctx context.Context, input *GetUsageInput) (*GetUsageOutput, error) {
	// Get API key ID and tier from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	tier, ok := GetAPIKeyTier(ctx)
	if !ok {
		tier = TierFree // Default to free tier if not set
	}

	// Get session counts
	dailyCount, err := a.db.GetDailySessionCount(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get daily session count", err)
	}

	activeCount, err := a.db.GetActiveSessionCount(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get active session count", err)
	}

	// Get tier limits
	limits := GetTierLimits(tier)

	// Calculate quota remaining
	quotaRemaining := limits.SessionsPerDay - dailyCount
	if IsUnlimited(limits.SessionsPerDay) {
		quotaRemaining = -1 // Indicate unlimited
	} else if quotaRemaining < 0 {
		quotaRemaining = 0
	}

	// Build response
	response := UsageResponse{
		SessionsToday:      dailyCount,
		ActiveSessions:     activeCount,
		QuotaUsed:          dailyCount,
		QuotaRemaining:     quotaRemaining,
		Tier:               tier,
		ConcurrentLimit:    limits.ConcurrentSessions,
		DailyLimit:         limits.SessionsPerDay,
		MaxDurationSeconds: limits.MaxDurationSec,
		MaxMemoryMB:        limits.MemoryMB,
	}

	return &GetUsageOutput{
		Body: response,
	}, nil
}

// JoinWaitlist handles POST /v1/waitlist
// Joins the waitlist and creates an API key (public endpoint, no auth required).
func (a *AccountService) JoinWaitlist(ctx context.Context, input *JoinWaitlistInput) (*JoinWaitlistOutput, error) {
	// Validate required fields
	if input.Body.Email == "" {
		return nil, huma.Error400BadRequest("email is required")
	}

	// Create API key in database
	apiKey, err := a.db.CreateAPIKey(ctx, input.Body.Email, input.Body.Name)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create API key", err)
	}

	// Build response
	response := WaitlistResponse{
		ID:      apiKey.ID.String(),
		Key:     apiKey.Key,
		Tier:    apiKey.Tier,
		Message: "Welcome to execbox! Save your API key - it will only be shown once.",
	}

	return &JoinWaitlistOutput{
		Body: response,
	}, nil
}

// GetEnhancedUsage handles GET /v1/account/usage/enhanced
// Returns enhanced usage statistics with detailed metrics for the authenticated API key.
func (a *AccountService) GetEnhancedUsage(ctx context.Context, input *GetEnhancedUsageInput) (*GetEnhancedUsageOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get tier from context
	tier, ok := GetAPIKeyTier(ctx)
	if !ok {
		tier = TierFree // Default to free tier if not set
	}

	// Get limits using tier
	limits := GetTierLimits(tier)

	// Get daily and active counts
	dailyCount, err := a.db.GetDailySessionCount(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get daily session count", err)
	}

	activeCount, err := a.db.GetActiveSessionCount(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get active session count", err)
	}

	// Calculate quota remaining
	quotaRemaining := limits.SessionsPerDay - dailyCount
	if IsUnlimited(limits.SessionsPerDay) {
		quotaRemaining = -1 // Indicate unlimited
	} else if quotaRemaining < 0 {
		quotaRemaining = 0
	}

	// Build base UsageResponse
	usageResponse := UsageResponse{
		SessionsToday:      dailyCount,
		ActiveSessions:     activeCount,
		QuotaUsed:          dailyCount,
		QuotaRemaining:     quotaRemaining,
		Tier:               tier,
		ConcurrentLimit:    limits.ConcurrentSessions,
		DailyLimit:         limits.SessionsPerDay,
		MaxDurationSeconds: limits.MaxDurationSec,
		MaxMemoryMB:        limits.MemoryMB,
	}

	// Get hourly usage for last 24 hours
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	hourlyUsage, err := a.db.GetHourlyAccountUsage(ctx, apiKeyID, start, now)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get hourly account usage", err)
	}

	// Convert to API format
	var hourlyAPIUsage []HourlyUsage
	for _, h := range hourlyUsage {
		hourlyAPIUsage = append(hourlyAPIUsage, HourlyUsage{
			Hour:       h.Hour.Format("2006-01-02T15:04:05Z07:00"),
			Executions: h.Executions,
			CostCents:  h.CostEstimateCents,
			Errors:     h.Errors,
		})
	}

	// Get daily usage history
	dailyUsage, err := a.db.GetDailyAccountUsage(ctx, apiKeyID, input.Days)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get daily account usage", err)
	}

	// Convert to API format and calculate cost estimate
	var dailyAPIUsage []DayUsage
	var totalCostEstimate int64
	for _, d := range dailyUsage {
		// Use consistent cost calculation - estimate with duration-based CPU usage
		costCents := DefaultCostCalculator.CalculateSessionCost(d.DurationMs, d.DurationMs, 256)
		totalCostEstimate += costCents

		// Aggregate errors from hourly usage for this day
		dayErrors := 0
		for _, h := range hourlyUsage {
			if h.Hour.Year() == d.Date.Year() && h.Hour.Month() == d.Date.Month() && h.Hour.Day() == d.Date.Day() {
				dayErrors += h.Errors
			}
		}

		dailyAPIUsage = append(dailyAPIUsage, DayUsage{
			Date:       d.Date.Format("2006-01-02"),
			Executions: d.Executions,
			DurationMs: d.DurationMs,
			CostCents:  costCents,
			Errors:     dayErrors,
		})
	}

	// Build enhanced response
	response := EnhancedUsageResponse{
		UsageResponse:     usageResponse,
		AccountID:         apiKeyID.String(),
		HourlyUsage:       hourlyAPIUsage,
		DailyHistory:      dailyAPIUsage,
		CostEstimateCents: totalCostEstimate,
		AlertThreshold:    80, // Default
	}

	return &GetEnhancedUsageOutput{
		Body: response,
	}, nil
}

// GetAccountLimits handles GET /v1/account/limits
// Returns account limits for the authenticated API key.
func (a *AccountService) GetAccountLimits(ctx context.Context, input *GetAccountLimitsInput) (*GetAccountLimitsOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get existing limits
	limits, err := a.db.GetAccountLimits(ctx, apiKeyID)
	if err != nil {
		// If not found, return default limits based on tier
		if strings.Contains(err.Error(), "not found") {
			tier, ok := GetAPIKeyTier(ctx)
			if !ok {
				tier = TierFree
			}
			tierLimits := GetTierLimits(tier)

			response := AccountLimitsResponse{
				DailyRequestsLimit:      tierLimits.SessionsPerDay,
				ConcurrentRequestsLimit: tierLimits.ConcurrentSessions,
				AlertThreshold:          80, // Default
				Timezone:                "UTC",
			}
			return &GetAccountLimitsOutput{
				Body: response,
			}, nil
		}
		return nil, huma.Error500InternalServerError("failed to get account limits", err)
	}

	// Build response from database limits
	response := AccountLimitsResponse{
		DailyRequestsLimit:      limits.DailyRequestsLimit,
		ConcurrentRequestsLimit: limits.ConcurrentRequestsLimit,
		MonthlyCostLimitCents:   limits.MonthlyCostLimitCents,
		AlertThreshold:          limits.AlertThresholdPercentage,
		BillingEmail:            limits.BillingEmail,
		Timezone:                limits.Timezone,
	}

	return &GetAccountLimitsOutput{
		Body: response,
	}, nil
}

// UpdateAccountLimits handles PUT /v1/account/limits
// Updates account limits for the authenticated API key.
func (a *AccountService) UpdateAccountLimits(ctx context.Context, input *UpdateAccountLimitsInput) (*UpdateAccountLimitsOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get existing limits first
	getOutput, err := a.GetAccountLimits(ctx, &GetAccountLimitsInput{})
	if err != nil {
		return nil, err
	}
	existing := getOutput.Body

	// Build updated limits, applying only non-nil fields
	limits := &db.AccountLimits{
		AccountID:                apiKeyID,
		DailyRequestsLimit:       existing.DailyRequestsLimit,
		ConcurrentRequestsLimit:  existing.ConcurrentRequestsLimit,
		MonthlyCostLimitCents:    existing.MonthlyCostLimitCents,
		AlertThresholdPercentage: existing.AlertThreshold,
		BillingEmail:             existing.BillingEmail,
		Timezone:                 existing.Timezone,
		UpdatedAt:                time.Now().UTC(),
		CreatedAt:                time.Now().UTC(), // Will be set by database
	}

	// Apply updates from input
	if input.Body.DailyRequestsLimit != nil {
		limits.DailyRequestsLimit = *input.Body.DailyRequestsLimit
	}
	if input.Body.ConcurrentRequestsLimit != nil {
		limits.ConcurrentRequestsLimit = *input.Body.ConcurrentRequestsLimit
	}
	if input.Body.MonthlyCostLimitCents != nil {
		limits.MonthlyCostLimitCents = input.Body.MonthlyCostLimitCents
	}
	if input.Body.AlertThreshold != nil {
		limits.AlertThresholdPercentage = *input.Body.AlertThreshold
	}
	if input.Body.BillingEmail != nil {
		limits.BillingEmail = input.Body.BillingEmail
	}
	if input.Body.Timezone != nil {
		limits.Timezone = *input.Body.Timezone
	}

	// Update in database
	err = a.db.UpsertAccountLimits(ctx, limits)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update account limits", err)
	}

	// Return updated limits
	response := AccountLimitsResponse{
		DailyRequestsLimit:      limits.DailyRequestsLimit,
		ConcurrentRequestsLimit: limits.ConcurrentRequestsLimit,
		MonthlyCostLimitCents:   limits.MonthlyCostLimitCents,
		AlertThreshold:          limits.AlertThresholdPercentage,
		BillingEmail:            limits.BillingEmail,
		Timezone:                limits.Timezone,
	}

	return &UpdateAccountLimitsOutput{
		Body: response,
	}, nil
}

// ExportUsage handles GET /v1/account/usage/export
// Returns usage data in exportable format for the authenticated API key.
func (a *AccountService) ExportUsage(ctx context.Context, input *ExportUsageInput) (*ExportUsageOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get daily usage history
	dailyUsage, err := a.db.GetDailyAccountUsage(ctx, apiKeyID, input.Days)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get daily account usage", err)
	}

	// Get hourly usage for error calculation
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -input.Days) // Go back the requested number of days
	hourlyUsage, err := a.db.GetHourlyAccountUsage(ctx, apiKeyID, start, now)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get hourly account usage", err)
	}

	// Convert to API format
	var dailyAPIUsage []DayUsage
	for _, d := range dailyUsage {
		// Use consistent cost calculation - estimate with duration-based CPU usage
		costCents := DefaultCostCalculator.CalculateSessionCost(d.DurationMs, d.DurationMs, 256)

		// Aggregate errors from hourly usage for this day
		dayErrors := 0
		for _, h := range hourlyUsage {
			if h.Hour.Year() == d.Date.Year() && h.Hour.Month() == d.Date.Month() && h.Hour.Day() == d.Date.Day() {
				dayErrors += h.Errors
			}
		}

		dailyAPIUsage = append(dailyAPIUsage, DayUsage{
			Date:       d.Date.Format("2006-01-02"),
			Executions: d.Executions,
			DurationMs: d.DurationMs,
			CostCents:  costCents,
			Errors:     dayErrors,
		})
	}

	return &ExportUsageOutput{
		Body: dailyAPIUsage,
	}, nil
}
