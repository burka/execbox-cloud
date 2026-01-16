package api

import (
	"context"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/danielgtaylor/huma/v2"
)

// QuotaService handles quota request operations.
type QuotaService struct {
	db DBClient
}

// NewQuotaService creates a new QuotaService.
func NewQuotaService(db DBClient) *QuotaService {
	return &QuotaService{db: db}
}

// CreateQuotaRequest handles POST /v1/quota-requests
// Creates a new quota request for users wanting to upgrade their tier.
// This endpoint is public (no auth required) but can optionally be used with auth.
func (q *QuotaService) CreateQuotaRequest(ctx context.Context, input *CreateQuotaRequestInput) (*CreateQuotaRequestOutput, error) {
	req := input.Body

	// Validate required fields
	if req.Email == "" {
		return nil, huma.Error400BadRequest("email is required")
	}

	// Build quota request model
	quotaReq := &db.QuotaRequest{
		Email:           req.Email,
		Name:            req.Name,
		Company:         req.Company,
		UseCase:         req.UseCase,
		RequestedLimits: req.RequestedLimits,
		Budget:          req.Budget,
	}

	// If authenticated, attach API key info
	if apiKeyID, ok := GetAPIKeyID(ctx); ok {
		quotaReq.APIKeyID = &apiKeyID

		// Try to get current tier from context
		if tier, ok := GetAPIKeyTier(ctx); ok {
			quotaReq.CurrentTier = &tier
		}
	}

	// Create in database
	created, err := q.db.CreateQuotaRequest(ctx, quotaReq)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create quota request", err)
	}

	// Build response
	return &CreateQuotaRequestOutput{
		Body: QuotaRequestResponse{
			ID:        created.ID,
			Status:    QuotaStatusPending,
			Message:   "Your quota request has been submitted. We'll review it and get back to you soon.",
			CreatedAt: created.CreatedAt.Format(time.RFC3339),
		},
	}, nil
}
