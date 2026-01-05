package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/burka/execbox-cloud/static"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the HTTP server with all dependencies.
type Server struct {
	router      *chi.Mux
	handlers    *Handlers
	db          *db.Client
	fly         *fly.Client
	rateLimiter *RateLimiter
	config      *Config
}

// Config holds all configuration for the server.
type Config struct {
	Port        string
	DatabaseURL string
	FlyToken    string
	FlyOrg      string
	FlyAppName  string
	LogLevel    string
}

// NewServer creates and configures a new server instance.
// It initializes the database client, Fly client, handlers, and sets up all routes.
func NewServer(cfg *Config) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// 1. Create database client
	dbClient, err := db.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create database client: %w", err)
	}

	// 2. Run migrations
	slog.Info("running database migrations")
	if err := dbClient.RunMigrations(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// 3. Create Fly client
	flyClient := fly.New(cfg.FlyToken, cfg.FlyOrg, cfg.FlyAppName)

	// 4. Create handlers
	handlers := NewHandlers(dbClient, flyClient)

	// 5. Set up image builder and cache
	builder := fly.NewBuilder(flyClient, cfg.FlyAppName)
	cache := fly.NewDBBuildCache(
		dbClient.GetImageCache,
		dbClient.PutImageCache,
		dbClient.TouchImageCache,
	)
	handlers.SetBuilder(builder, cache)

	// 6. Create rate limiter
	rateLimiter := NewRateLimiter()

	// 7. Set up chi router with middleware
	router := chi.NewRouter()

	// Global middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(RecoveryMiddleware)
	router.Use(LoggingMiddleware)

	// 8. Create server and register routes
	s := &Server{
		router:      router,
		handlers:    handlers,
		db:          dbClient,
		fly:         flyClient,
		rateLimiter: rateLimiter,
		config:      cfg,
	}

	s.registerRoutes()

	return s, nil
}

// registerRoutes sets up all HTTP routes.
func (s *Server) registerRoutes() {
	// Set up OpenAPI documentation
	_ = SetupOpenAPI(s.router)

	// Health check endpoint (no auth required)
	s.router.Get("/health", s.healthCheck)

	// API v1 routes
	s.router.Route("/v1", func(r chi.Router) {
		// Public endpoints with IP-based rate limiting
		r.Group(func(r chi.Router) {
			r.Use(s.rateLimiter.IPMiddleware())
			r.Post("/quota-requests", s.handlers.CreateQuotaRequest)
		})

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			// Apply auth middleware first, then rate limiting
			r.Use(AuthMiddleware(s.db))
			r.Use(s.rateLimiter.Middleware())

			// Session management
			r.Post("/sessions", s.handlers.CreateSession)
			r.Get("/sessions", s.handlers.ListSessions)
			r.Get("/sessions/{id}", s.handlers.GetSession)
			r.Post("/sessions/{id}/stop", s.handlers.StopSession)
			r.Delete("/sessions/{id}", s.handlers.KillSession)

			// WebSocket attach endpoint
			r.Get("/sessions/{id}/attach", s.handleAttach)
		})
	})

	// Dashboard SPA - catch all remaining routes
	// This must be registered AFTER all API routes to ensure API routes take precedence
	dashboardFS, err := static.DashboardFS()
	if err != nil {
		slog.Error("failed to get dashboard filesystem", "error", err)
	} else {
		spaHandler := NewSPAHandler(dashboardFS, "index.html", "/assets/")
		s.router.Handle("/*", spaHandler)
	}
}

// healthCheck handles GET /health - verifies server and database connectivity.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	// Check database health
	if err := s.db.Health(r.Context()); err != nil {
		slog.Error("health check failed", "error", err)
		WriteError(w, fmt.Errorf("database health check failed: %w", err),
			http.StatusServiceUnavailable, "UNAVAILABLE")
		return
	}

	// Return healthy status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleAttach wraps the WebSocket attach handler to extract session ID and API key.
// This is needed because AttachSession expects additional parameters beyond http.Handler signature.
func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request) {
	// Get session ID from URL params
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		WriteError(w, fmt.Errorf("%w: session ID is required", ErrBadRequest),
			http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Get API key ID from context (set by auth middleware)
	apiKeyID, ok := GetAPIKeyID(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Create API key struct with the ID for ownership check
	// AttachSession only needs the ID for ownership validation
	apiKey := &db.APIKey{ID: apiKeyID}

	// Call the attach handler
	s.handlers.AttachSession(w, r, sessionID, apiKey)
}

// Router returns the chi router instance for use with http.Server.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Close gracefully shuts down the server by closing the database connection.
func (s *Server) Close() error {
	if s.db != nil {
		s.db.Close()
	}
	return nil
}
