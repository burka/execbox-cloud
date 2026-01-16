package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/backend/k8s"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/burka/execbox-cloud/static"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server represents the HTTP server with all dependencies.
type Server struct {
	router      *chi.Mux
	services    *Services
	db          *db.Client
	fly         *fly.Client // Deprecated: Use backend instead
	backend     Backend
	rateLimiter *RateLimiter
	config      *Config
}

// Config holds all configuration for the server.
type Config struct {
	Port        string
	DatabaseURL string
	LogLevel    string

	// Backend selection
	Backend string // "fly" or "kubernetes"

	// Fly.io config (used when Backend="fly")
	FlyToken   string
	FlyOrg     string
	FlyAppName string

	// Kubernetes config (used when Backend="kubernetes")
	K8sKubeconfig     string
	K8sNamespace      string
	K8sServiceAccount string
	K8sRegistry       string
	K8sImageTTL       string
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

	// 3. Create backend based on configuration
	var backend Backend
	var flyClient *fly.Client

	switch cfg.Backend {
	case "fly":
		slog.Info("initializing Fly.io backend")
		flyClient = fly.New(cfg.FlyToken, cfg.FlyOrg, cfg.FlyAppName)
		backend = NewFlyBackend(flyClient)

	case "kubernetes":
		slog.Info("initializing Kubernetes backend",
			"namespace", cfg.K8sNamespace,
			"registry", cfg.K8sRegistry,
		)
		k8sBackend, err := k8s.NewBackend(k8s.BackendConfig{
			Kubeconfig:     cfg.K8sKubeconfig,
			Namespace:      cfg.K8sNamespace,
			ServiceAccount: cfg.K8sServiceAccount,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes backend: %w", err)
		}
		backend = NewK8sBackend(k8sBackend)

	default:
		return nil, fmt.Errorf("unknown backend: %s", cfg.Backend)
	}

	// 4. Create services with backend
	sessionService := NewSessionService(dbClient, backend)
	accountService := NewAccountService(dbClient)
	quotaService := NewQuotaService(dbClient)

	// 5. Set up image builder and cache (Fly-specific for now)
	if flyClient != nil {
		builder := fly.NewBuilder(flyClient, cfg.FlyAppName)
		cache := fly.NewDBBuildCache(
			dbClient.GetImageCache,
			dbClient.PutImageCache,
			dbClient.TouchImageCache,
		)
		sessionService.SetBuilder(builder, cache)
	}

	services := &Services{
		Session: sessionService,
		Account: accountService,
		Quota:   quotaService,
		DB:      dbClient,
	}

	// 6. Create rate limiter
	rateLimiter := NewRateLimiter()

	// 7. Set up chi router with middleware
	router := chi.NewRouter()

	// Global middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(RecoveryMiddleware)
	router.Use(LoggingMiddleware)

	// 8. Register huma routes (replaces chi routes)
	RegisterRoutes(router, services, rateLimiter)

	// 9. Register WebSocket attach endpoint (special handling - not a huma handler)
	router.Route("/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(dbClient))
			r.Use(rateLimiter.Middleware())
			r.Get("/sessions/{id}/attach", handleAttach(services.Session, dbClient))
		})
	})

	// 10. Dashboard SPA - catch all remaining routes
	dashboardFS, err := static.DashboardFS()
	if err != nil {
		slog.Error("failed to get dashboard filesystem", "error", err)
	} else {
		spaHandler := NewSPAHandler(dashboardFS, "index.html", "/assets/")
		router.Handle("/*", spaHandler)
	}

	// 11. Create server
	s := &Server{
		router:      router,
		services:    services,
		db:          dbClient,
		fly:         flyClient,
		backend:     backend,
		rateLimiter: rateLimiter,
		config:      cfg,
	}

	return s, nil
}

// handleAttach creates a handler that wraps WebSocket attach for session I/O streaming.
// This needs special handling because WebSocket upgrades don't fit the standard huma pattern.
func handleAttach(sessionSvc *SessionService, dbClient DBClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		apiKey := &db.APIKey{ID: apiKeyID}

		// Call the WebSocket attach handler
		// Note: This uses the Handlers struct's AttachSession for backward compatibility
		// TODO: Move WebSocket handling to SessionService when refactoring websocket.go
		handlers := &Handlers{db: dbClient, backend: sessionSvc.backend}
		handlers.AttachSession(w, r, sessionID, apiKey)
	}
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
