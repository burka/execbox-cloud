package api

import (
	"context"

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
