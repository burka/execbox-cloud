package api

import (
	"context"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/google/uuid"
)

// DBClient defines the database operations required by the API handlers
type DBClient interface {
	GetAPIKeyByKey(ctx context.Context, key string) (*db.APIKey, error)
	GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*db.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error
	CreateAPIKey(ctx context.Context, email string, name *string) (*db.APIKey, error)
	GetSession(ctx context.Context, id string) (*db.Session, error)
	CreateSession(ctx context.Context, sess *db.Session) error
	UpdateSession(ctx context.Context, id string, update *db.SessionUpdate) error
	ListSessions(ctx context.Context, apiKeyID uuid.UUID, status *string) ([]db.Session, error)
	DeleteSession(ctx context.Context, id string) error
	GetActiveSessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error)
	GetDailySessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error)
	CreateQuotaRequest(ctx context.Context, req *db.QuotaRequest) (*db.QuotaRequest, error)
}

// Ensure *db.Client implements DBClient interface
var _ DBClient = (*db.Client)(nil)
