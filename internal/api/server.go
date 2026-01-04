package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the HTTP server with all dependencies.
type Server struct {
	router   *chi.Mux
	handlers *Handlers
	db       *db.Client
	fly      *fly.Client
	config   *Config
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

	// 2. Create Fly client
	flyClient := fly.New(cfg.FlyToken, cfg.FlyOrg, cfg.FlyAppName)

	// 3. Create handlers
	handlers := NewHandlers(dbClient, flyClient)

	// 4. Set up chi router with middleware
	router := chi.NewRouter()

	// Global middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(RecoveryMiddleware)
	router.Use(LoggingMiddleware)

	// 5. Register routes
	s := &Server{
		router:   router,
		handlers: handlers,
		db:       dbClient,
		fly:      flyClient,
		config:   cfg,
	}

	s.registerRoutes()

	return s, nil
}

// registerRoutes sets up all HTTP routes.
func (s *Server) registerRoutes() {
	// Health check endpoint (no auth required)
	s.router.Get("/health", s.healthCheck)

	// API v1 routes with authentication
	s.router.Route("/v1", func(r chi.Router) {
		// Apply auth middleware to all v1 routes
		r.Use(AuthMiddleware(s.db))

		// Session management
		r.Post("/sessions", s.handlers.CreateSession)
		r.Get("/sessions", s.handlers.ListSessions)
		r.Get("/sessions/{id}", s.handlers.GetSession)
		r.Post("/sessions/{id}/stop", s.handlers.StopSession)
		r.Delete("/sessions/{id}", s.handlers.KillSession)

		// WebSocket attach endpoint
		r.Get("/sessions/{id}/attach", s.handleAttach)
	})
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

	// Look up the API key details
	apiKey, err := s.db.GetAPIKeyByKey(r.Context(), "") // We need to modify this approach
	if err != nil {
		// For now, create a minimal API key object with just the ID
		// TODO: Store full API key in context or adjust AttachSession signature
		apiKey = &db.APIKey{ID: apiKeyID}
	}

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
