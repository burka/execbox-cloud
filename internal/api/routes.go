package api

import (
	gocontext "context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// Services holds all the service instances used by the API.
type Services struct {
	Session *SessionService
	Account *AccountService
	Quota   *QuotaService
	DB      *db.Client
}

// RegisterRoutes registers all huma routes with their service handlers.
// It sets up the huma API with OpenAPI documentation and security schemes,
// then registers all endpoints with proper middleware.
func RegisterRoutes(router *chi.Mux, services *Services, rateLimiter *RateLimiter) huma.API {
	// Set up huma API with OpenAPI config
	config := huma.DefaultConfig("Execbox Cloud API", "1.0.0")
	config.Info = OpenAPIInfo().Info
	config.Tags = OpenAPIInfo().Tags
	config.Servers = OpenAPIInfo().Servers

	humaAPI := humachi.New(router, config)

	// Initialize security schemes
	if humaAPI.OpenAPI().Components.SecuritySchemes == nil {
		humaAPI.OpenAPI().Components.SecuritySchemes = make(map[string]*huma.SecurityScheme)
	}
	for name, scheme := range SecuritySchemes() {
		humaAPI.OpenAPI().Components.SecuritySchemes[name] = scheme
	}

	// Register OpenAPI spec endpoints (unauthenticated)
	router.Get("/openapi.json", handleOpenAPIJSON(humaAPI))
	router.Get("/openapi.yaml", handleOpenAPIYAML(humaAPI))

	// Health check (no auth, no rate limit)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "health",
		Method:      "GET",
		Path:        "/health",
		Summary:     "Health check",
		Description: "Returns server health status. Does not require authentication.",
		Tags:        []string{"Health"},
	}, func(ctx gocontext.Context, input *HealthCheckInput) (*HealthCheckOutput, error) {
		// When DB is nil, we're in spec-generation mode - return success for type extraction
		if services.DB != nil {
			if err := services.DB.Health(ctx); err != nil {
				return nil, huma.Error503ServiceUnavailable(fmt.Sprintf("database health check failed: %v", err))
			}
		}
		return &HealthCheckOutput{
			Body: struct {
				Status string `json:"status" doc:"Health status" example:"ok"`
			}{Status: "ok"},
		}, nil
	})

	// Public endpoints (IP rate limited, no auth)
	registerPublicRoutes(humaAPI, router, services, rateLimiter)

	// Authenticated endpoints (auth + rate limited)
	registerAuthenticatedRoutes(humaAPI, router, services, rateLimiter)

	return humaAPI
}

// registerPublicRoutes registers endpoints that don't require authentication.
func registerPublicRoutes(humaAPI huma.API, router *chi.Mux, services *Services, rateLimiter *RateLimiter) {
	// Create a middleware that applies IP rate limiting for public endpoints
	ipLimitedMiddleware := func(ctx huma.Context, next func(huma.Context)) {
		// Apply IP rate limiting through chi middleware
		// For huma, we need to apply this differently - we'll handle it per-request
		next(ctx)
	}

	// POST /v1/waitlist - Join waitlist (public)
	huma.Register(humaAPI, huma.Operation{
		OperationID:   "joinWaitlist",
		Method:        "POST",
		Path:          "/v1/waitlist",
		Summary:       "Join the waitlist",
		Description:   "Join the waitlist to get early access. Returns an API key immediately for the free tier.",
		Tags:          []string{"Waitlist"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{ipLimitedMiddleware},
	}, services.Account.JoinWaitlist)

	// POST /v1/quota-requests - Create quota request (public)
	huma.Register(humaAPI, huma.Operation{
		OperationID:   "createQuotaRequest",
		Method:        "POST",
		Path:          "/v1/quota-requests",
		Summary:       "Request quota increase",
		Description:   "Submit a request to increase API usage limits. Does not require authentication.",
		Tags:          []string{"Quota"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{ipLimitedMiddleware},
	}, services.Quota.CreateQuotaRequest)
}

// registerAuthenticatedRoutes registers endpoints that require authentication.
func registerAuthenticatedRoutes(humaAPI huma.API, router *chi.Mux, services *Services, rateLimiter *RateLimiter) {
	// Auth middleware for huma
	authMiddleware := humaAuthMiddleware(services.DB)

	securityRequirement := []map[string][]string{{"bearerAuth": {}}}

	// Account operations
	huma.Register(humaAPI, huma.Operation{
		OperationID: "getAccount",
		Method:      "GET",
		Path:        "/v1/account",
		Summary:     "Get account information",
		Description: "Returns account information including tier, email, and API key details.",
		Tags:        []string{"Account"},
		Security:    securityRequirement,
		Middlewares: huma.Middlewares{authMiddleware},
	}, services.Account.GetAccount)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "getUsage",
		Method:      "GET",
		Path:        "/v1/account/usage",
		Summary:     "Get usage statistics",
		Description: "Returns usage statistics including sessions today, quota remaining, and limits.",
		Tags:        []string{"Account"},
		Security:    securityRequirement,
		Middlewares: huma.Middlewares{authMiddleware},
	}, services.Account.GetUsage)

	// Session operations
	huma.Register(humaAPI, huma.Operation{
		OperationID:   "createSession",
		Method:        "POST",
		Path:          "/v1/sessions",
		Summary:       "Create a new session",
		Description:   "Create a new execution session with the specified container image and configuration.",
		Tags:          []string{"Sessions"},
		Security:      securityRequirement,
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{authMiddleware},
	}, services.Session.CreateSession)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "listSessions",
		Method:      "GET",
		Path:        "/v1/sessions",
		Summary:     "List all sessions",
		Description: "Returns a list of all active and recently completed sessions for the authenticated user.",
		Tags:        []string{"Sessions"},
		Security:    securityRequirement,
		Middlewares: huma.Middlewares{authMiddleware},
	}, services.Session.ListSessions)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "getSession",
		Method:      "GET",
		Path:        "/v1/sessions/{id}",
		Summary:     "Get session info",
		Description: "Returns detailed information about a specific session.",
		Tags:        []string{"Sessions"},
		Security:    securityRequirement,
		Middlewares: huma.Middlewares{authMiddleware},
	}, services.Session.GetSession)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "stopSession",
		Method:      "POST",
		Path:        "/v1/sessions/{id}/stop",
		Summary:     "Stop a session",
		Description: "Gracefully stop a running session.",
		Tags:        []string{"Sessions"},
		Security:    securityRequirement,
		Middlewares: huma.Middlewares{authMiddleware},
	}, services.Session.StopSession)

	huma.Register(humaAPI, huma.Operation{
		OperationID:   "killSession",
		Method:        "DELETE",
		Path:          "/v1/sessions/{id}",
		Summary:       "Kill a session",
		Description:   "Forcefully terminate a session immediately.",
		Tags:          []string{"Sessions"},
		Security:      securityRequirement,
		DefaultStatus: 200,
		Middlewares:   huma.Middlewares{authMiddleware},
	}, services.Session.KillSession)

	// Note: WebSocket attach endpoint (/v1/sessions/{id}/attach) is registered
	// via chi directly in server.go because WebSocket upgrades don't work well
	// with huma's response handling. OpenAPI docs for it should be added manually
	// or via a separate schema definition.
}

// humaAuthMiddleware creates a huma middleware that validates the API key
// and sets the API key ID and tier in the context.
func humaAuthMiddleware(dbClient DBClient) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// Get the Authorization header
		authHeader := ctx.Header("Authorization")
		if authHeader == "" {
			writeHumaUnauthorized(ctx, "missing authorization header")
			return
		}

		// Extract the API key from "Bearer <key>" format
		if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			writeHumaUnauthorized(ctx, "invalid authorization header format")
			return
		}
		apiKey := authHeader[7:]

		// Validate the API key
		key, err := dbClient.GetAPIKeyByKey(ctx.Context(), apiKey)
		if err != nil {
			writeHumaUnauthorized(ctx, "invalid API key")
			return
		}

		// Set API key info in context
		newCtx := WithAPIKeyID(ctx.Context(), key.ID)
		newCtx = WithAPIKeyTier(newCtx, key.Tier)

		// Create a new context wrapper with the updated context
		next(&humaContextWrapper{inner: ctx, overrideCtx: newCtx})
	}
}

// writeHumaUnauthorized writes a 401 Unauthorized response for huma middleware.
func writeHumaUnauthorized(ctx huma.Context, msg string) {
	ctx.SetStatus(http.StatusUnauthorized)
	ctx.SetHeader("Content-Type", "application/json")
	_, _ = ctx.BodyWriter().Write([]byte(fmt.Sprintf(`{"error":"%s"}`, msg)))
}

// humaContextWrapper wraps a huma.Context with a custom gocontext.Context.
type humaContextWrapper struct {
	inner       huma.Context
	overrideCtx gocontext.Context //nolint:containedctx // Required to override embedded huma.Context
}

// Implement all huma.Context methods by delegating to inner, except Context()
func (c *humaContextWrapper) Context() gocontext.Context              { return c.overrideCtx }
func (c *humaContextWrapper) Operation() *huma.Operation              { return c.inner.Operation() }
func (c *humaContextWrapper) TLS() *tls.ConnectionState               { return c.inner.TLS() }
func (c *humaContextWrapper) Version() huma.ProtoVersion              { return c.inner.Version() }
func (c *humaContextWrapper) Method() string                          { return c.inner.Method() }
func (c *humaContextWrapper) Host() string                            { return c.inner.Host() }
func (c *humaContextWrapper) RemoteAddr() string                      { return c.inner.RemoteAddr() }
func (c *humaContextWrapper) URL() url.URL                            { return c.inner.URL() }
func (c *humaContextWrapper) Param(name string) string                { return c.inner.Param(name) }
func (c *humaContextWrapper) Query(name string) string                { return c.inner.Query(name) }
func (c *humaContextWrapper) Header(name string) string               { return c.inner.Header(name) }
func (c *humaContextWrapper) EachHeader(cb func(name, value string))  { c.inner.EachHeader(cb) }
func (c *humaContextWrapper) BodyReader() io.Reader                   { return c.inner.BodyReader() }
func (c *humaContextWrapper) GetMultipartForm() (*multipart.Form, error) { return c.inner.GetMultipartForm() }
func (c *humaContextWrapper) SetReadDeadline(t time.Time) error       { return c.inner.SetReadDeadline(t) }
func (c *humaContextWrapper) SetStatus(code int)                      { c.inner.SetStatus(code) }
func (c *humaContextWrapper) Status() int                             { return c.inner.Status() }
func (c *humaContextWrapper) SetHeader(name, value string)            { c.inner.SetHeader(name, value) }
func (c *humaContextWrapper) AppendHeader(name, value string)         { c.inner.AppendHeader(name, value) }
func (c *humaContextWrapper) BodyWriter() io.Writer                   { return c.inner.BodyWriter() }
