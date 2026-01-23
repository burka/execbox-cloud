package api

import (
	"context"
	"strings"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// parseUUID parses a string into a UUID.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

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

// ============================================================================
// API Key Management
// ============================================================================

// ListAPIKeys handles GET /v1/account/keys
// Returns all API keys for the authenticated account.
func (a *AccountService) ListAPIKeys(ctx context.Context, input *ListAPIKeysInput) (*ListAPIKeysOutput, error) {
	// Get API key ID from context (used as account ID for primary keys)
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get the current API key to find the account ID
	currentKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get current API key", err)
	}

	// Get all keys for this account
	keys, err := a.db.GetAPIKeysByAccount(ctx, currentKey.AccountID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get API keys", err)
	}

	// Convert to API response format
	var keyResponses []APIKeyResponse
	for _, k := range keys {
		keyResp := apiKeyToResponse(&k)
		keyResponses = append(keyResponses, keyResp)
	}

	return &ListAPIKeysOutput{
		Body: ListAPIKeysResponse{Keys: keyResponses},
	}, nil
}

// CreateAPIKey handles POST /v1/account/keys
// Creates a new API key for the authenticated account.
// If configuration fails after creation, the new key is deactivated to prevent orphaned state.
func (a *AccountService) CreateAPIKey(ctx context.Context, input *CreateAPIKeyInput) (*CreateAPIKeyOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Validate required fields
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}

	// Validate name length
	if len(input.Body.Name) > 100 {
		return nil, huma.Error400BadRequest("name must be 100 characters or less")
	}

	// Validate description length if provided
	if input.Body.Description != nil && len(*input.Body.Description) > 500 {
		return nil, huma.Error400BadRequest("description must be 500 characters or less")
	}

	// Validate custom daily limit if provided
	if input.Body.CustomDailyLimit != nil && *input.Body.CustomDailyLimit <= 0 {
		return nil, huma.Error400BadRequest("custom_daily_limit must be greater than 0")
	}

	// Validate custom concurrent limit if provided
	if input.Body.CustomConcurrentLimit != nil && *input.Body.CustomConcurrentLimit <= 0 {
		return nil, huma.Error400BadRequest("custom_concurrent_limit must be greater than 0")
	}

	// Get the current API key to find the account ID
	currentKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get current API key", err)
	}

	// Get description
	description := ""
	if input.Body.Description != nil {
		description = *input.Body.Description
	}

	// Create the new key
	newKey, err := a.db.CreateAPIKeyForAccount(ctx, currentKey.AccountID, input.Body.Name, description, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create API key", err)
	}

	// Apply optional settings if provided
	if input.Body.ExpiresAt != nil || input.Body.CustomDailyLimit != nil || input.Body.CustomConcurrentLimit != nil {
		update := &db.APIKeyUpdate{}
		if input.Body.ExpiresAt != nil {
			expiresAt, err := time.Parse(time.RFC3339, *input.Body.ExpiresAt)
			if err != nil {
				// Cleanup: deactivate the newly created key
				_ = a.db.DeactivateAPIKey(ctx, newKey.ID, "system-cleanup: invalid expires_at format")
				return nil, huma.Error400BadRequest("invalid expires_at format, use RFC3339")
			}
			update.ExpiresAt = &expiresAt
		}
		if input.Body.CustomDailyLimit != nil {
			update.CustomDailyLimit = input.Body.CustomDailyLimit
		}
		if input.Body.CustomConcurrentLimit != nil {
			update.CustomConcurrentLimit = input.Body.CustomConcurrentLimit
		}

		// Apply the update and handle cleanup on failure
		if err := a.db.UpdateAPIKey(ctx, newKey.ID, update); err != nil {
			// Cleanup: deactivate the newly created key to prevent orphaned state
			_ = a.db.DeactivateAPIKey(ctx, newKey.ID, "system-cleanup: configuration failed")
			return nil, huma.Error500InternalServerError("failed to configure API key", err)
		}

		// Refresh the key data after update
		newKey, err = a.db.GetAPIKeyByID(ctx, newKey.ID)
		if err != nil {
			// Cleanup: deactivate the newly created key if refresh fails
			_ = a.db.DeactivateAPIKey(ctx, newKey.ID, "system-cleanup: failed to refresh")
			return nil, huma.Error500InternalServerError("failed to refresh API key", err)
		}
	}

	// Build response with full key visible
	keyResp := apiKeyToResponse(newKey)
	return &CreateAPIKeyOutput{
		Body: CreateAPIKeyResponse{
			APIKeyResponse: keyResp,
			Key:            newKey.Key, // Full key only shown once
		},
	}, nil
}

// GetAPIKey handles GET /v1/account/keys/{id}
// Returns a specific API key for the authenticated account.
func (a *AccountService) GetAPIKey(ctx context.Context, input *GetAPIKeyInput) (*GetAPIKeyOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get the current API key to find the account ID
	currentKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get current API key", err)
	}

	// Parse the target key ID
	targetKeyID, err := parseUUID(input.ID)
	if err != nil {
		return nil, huma.Error400BadRequest("invalid API key ID format")
	}

	// Get the target key
	targetKey, err := a.db.GetAPIKeyByID(ctx, targetKeyID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, huma.Error404NotFound("API key not found")
		}
		return nil, huma.Error500InternalServerError("failed to get API key", err)
	}

	// Verify the key belongs to the same account
	if targetKey.AccountID != currentKey.AccountID {
		return nil, huma.Error404NotFound("API key not found")
	}

	return &GetAPIKeyOutput{
		Body: apiKeyToResponse(targetKey),
	}, nil
}

// UpdateAPIKey handles PUT /v1/account/keys/{id}
// Updates an API key for the authenticated account.
func (a *AccountService) UpdateAPIKey(ctx context.Context, input *UpdateAPIKeyInput) (*UpdateAPIKeyOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Validate name length if provided
	if input.Body.Name != nil && len(*input.Body.Name) > 100 {
		return nil, huma.Error400BadRequest("name must be 100 characters or less")
	}

	// Validate description length if provided
	if input.Body.Description != nil && len(*input.Body.Description) > 500 {
		return nil, huma.Error400BadRequest("description must be 500 characters or less")
	}

	// Validate custom daily limit if provided
	if input.Body.CustomDailyLimit != nil && *input.Body.CustomDailyLimit <= 0 {
		return nil, huma.Error400BadRequest("custom_daily_limit must be greater than 0")
	}

	// Validate custom concurrent limit if provided
	if input.Body.CustomConcurrentLimit != nil && *input.Body.CustomConcurrentLimit <= 0 {
		return nil, huma.Error400BadRequest("custom_concurrent_limit must be greater than 0")
	}

	// Get the current API key to find the account ID
	currentKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get current API key", err)
	}

	// Parse the target key ID
	targetKeyID, err := parseUUID(input.ID)
	if err != nil {
		return nil, huma.Error400BadRequest("invalid API key ID format")
	}

	// Get the target key
	targetKey, err := a.db.GetAPIKeyByID(ctx, targetKeyID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, huma.Error404NotFound("API key not found")
		}
		return nil, huma.Error500InternalServerError("failed to get API key", err)
	}

	// Verify the key belongs to the same account
	if targetKey.AccountID != currentKey.AccountID {
		return nil, huma.Error404NotFound("API key not found")
	}

	// Build update from input
	update := &db.APIKeyUpdate{}
	hasUpdates := false

	if input.Body.Name != nil {
		update.Name = input.Body.Name
		hasUpdates = true
	}
	if input.Body.Description != nil {
		update.Description = input.Body.Description
		hasUpdates = true
	}
	if input.Body.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *input.Body.ExpiresAt)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid expires_at format, use RFC3339")
		}
		update.ExpiresAt = &expiresAt
		hasUpdates = true
	}
	if input.Body.CustomDailyLimit != nil {
		update.CustomDailyLimit = input.Body.CustomDailyLimit
		hasUpdates = true
	}
	if input.Body.CustomConcurrentLimit != nil {
		update.CustomConcurrentLimit = input.Body.CustomConcurrentLimit
		hasUpdates = true
	}

	if !hasUpdates {
		return nil, huma.Error400BadRequest("no fields to update")
	}

	// Track who made the update
	updatedBy := currentKey.ID.String()
	update.LastUpdatedBy = &updatedBy

	// Apply update
	if err := a.db.UpdateAPIKey(ctx, targetKeyID, update); err != nil {
		return nil, huma.Error500InternalServerError("failed to update API key", err)
	}

	// Get updated key
	updatedKey, err := a.db.GetAPIKeyByID(ctx, targetKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get updated API key", err)
	}

	return &UpdateAPIKeyOutput{
		Body: apiKeyToResponse(updatedKey),
	}, nil
}

// DeleteAPIKey handles DELETE /v1/account/keys/{id}
// Deactivates an API key for the authenticated account.
func (a *AccountService) DeleteAPIKey(ctx context.Context, input *DeleteAPIKeyInput) (*DeleteAPIKeyOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get the current API key to find the account ID
	currentKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get current API key", err)
	}

	// Parse the target key ID
	targetKeyID, err := parseUUID(input.ID)
	if err != nil {
		return nil, huma.Error400BadRequest("invalid API key ID format")
	}

	// Get the target key
	targetKey, err := a.db.GetAPIKeyByID(ctx, targetKeyID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, huma.Error404NotFound("API key not found")
		}
		return nil, huma.Error500InternalServerError("failed to get API key", err)
	}

	// Verify the key belongs to the same account
	if targetKey.AccountID != currentKey.AccountID {
		return nil, huma.Error404NotFound("API key not found")
	}

	// Check if this is the primary key
	isPrimary, err := a.db.IsPrimaryKey(ctx, targetKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check primary key status", err)
	}
	if isPrimary {
		return nil, huma.Error400BadRequest("cannot delete the primary API key")
	}

	// Deactivate the key (soft delete)
	performedBy := currentKey.ID.String()
	if err := a.db.DeactivateAPIKey(ctx, targetKeyID, performedBy); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete API key", err)
	}

	return &DeleteAPIKeyOutput{}, nil
}

// RotateAPIKey handles POST /v1/account/keys/{id}/rotate
// Rotates an API key, generating a new key value while preserving settings.
func (a *AccountService) RotateAPIKey(ctx context.Context, input *RotateAPIKeyInput) (*RotateAPIKeyOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get the current API key to find the account ID
	currentKey, err := a.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get current API key", err)
	}

	// Parse the target key ID
	targetKeyID, err := parseUUID(input.ID)
	if err != nil {
		return nil, huma.Error400BadRequest("invalid API key ID format")
	}

	// Get the target key to verify ownership
	targetKey, err := a.db.GetAPIKeyByID(ctx, targetKeyID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, huma.Error404NotFound("API key not found")
		}
		return nil, huma.Error500InternalServerError("failed to get API key", err)
	}

	// Verify the key belongs to the same account
	if targetKey.AccountID != currentKey.AccountID {
		return nil, huma.Error404NotFound("API key not found")
	}

	// Rotate the key
	performedBy := currentKey.ID.String()
	rotatedKey, err := a.db.RotateAPIKey(ctx, targetKeyID, performedBy)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to rotate API key", err)
	}

	// Build response with full key visible
	keyResp := apiKeyToResponse(rotatedKey)
	return &RotateAPIKeyOutput{
		Body: RotateAPIKeyResponse{
			APIKeyResponse: keyResp,
			Key:            rotatedKey.Key, // New key only shown once
		},
	}, nil
}

// apiKeyToResponse converts a db.APIKey to an APIKeyResponse.
func apiKeyToResponse(k *db.APIKey) APIKeyResponse {
	resp := APIKeyResponse{
		ID:                    k.ID.String(),
		Name:                  k.Name,
		Description:           k.Description,
		KeyPreview:            maskAPIKey(k.Key),
		IsActive:              k.IsActive,
		CustomDailyLimit:      k.CustomDailyLimit,
		CustomConcurrentLimit: k.CustomConcurrentLimit,
		CreatedAt:             k.CreatedAt.Format(time.RFC3339),
	}

	if k.ExpiresAt != nil {
		expiresAt := k.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expiresAt
	}

	if k.LastUsedAt != nil {
		lastUsedAt := k.LastUsedAt.Format(time.RFC3339)
		resp.LastUsedAt = &lastUsedAt
	}

	return resp
}
