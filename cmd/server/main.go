package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/burka/execbox-cloud/internal/api"
	"github.com/joho/godotenv"
)

func main() {
	// Parse command line flags
	openAPIFlag := flag.Bool("openapi", false, "Generate OpenAPI spec and exit")
	flag.Parse()

	// Handle --openapi flag
	if *openAPIFlag {
		// Load .env files first for database connection
		loadEnvFiles()
		spec, err := api.GenerateOpenAPISpec()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate OpenAPI spec: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(spec))
		os.Exit(0)
	}

	// 1. Load .env files and configuration from environment
	loadEnvFiles()
	cfg := loadConfig()

	// 2. Validate configuration before proceeding
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// 3. Set up slog with configured log level
	setupLogging(cfg.LogLevel)

	slog.Info("starting execbox-cloud server",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
	)

	// 4. Create server
	server, err := api.NewServer(cfg)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	// 5. Set up HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 6. Handle graceful shutdown
	// Create channel to listen for errors from the HTTP server
	serverErrors := make(chan error, 1)

	// Start HTTP server in a goroutine
	go func() {
		slog.Info("server listening", "addr", httpServer.Addr)
		serverErrors <- httpServer.ListenAndServe()
	}()

	// Create channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}

	case sig := <-shutdown:
		slog.Info("shutdown signal received", "signal", sig.String())

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			// Force close
			if err := httpServer.Close(); err != nil {
				slog.Error("server close failed", "error", err)
			}
			os.Exit(1)
		}

		slog.Info("server stopped gracefully")
	}
}

// validateConfig checks that required configuration values are set.
func validateConfig(cfg *api.Config) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.FlyToken == "" {
		return fmt.Errorf("FLY_API_TOKEN is required")
	}
	return nil
}

// loadEnvFiles loads environment variables from .env files.
// It looks for .env.local, .env.development, then .env (in that order).
func loadEnvFiles() {
	// Try to load .env.local first (highest priority)
	if err := godotenv.Load(".env.local"); err == nil {
		slog.Debug("loaded .env.local")
	}

	// Try to load .env.development for development environment
	if err := godotenv.Load(".env.development"); err == nil {
		slog.Debug("loaded .env.development")
	}

	// Finally load .env (fallback)
	if err := godotenv.Load(".env"); err == nil {
		slog.Debug("loaded .env")
	}
}

// loadConfig reads configuration from environment variables with sensible defaults.
func loadConfig() *api.Config {
	// Calculate development ports: Use 20000 + current port if not explicitly set
	devPort := calculateDevPort(getEnv("PORT", ""))

	return &api.Config{
		Port:        devPort,
		DatabaseURL: getEnv("DATABASE_URL", ""),
		FlyToken:    getEnv("FLY_API_TOKEN", ""),
		FlyOrg:      getEnv("FLY_ORG", ""),
		FlyAppName:  getEnv("FLY_APP_NAME", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}
}

// calculateDevPort returns a high port number for development.
// If PORT is explicitly set, use it. Otherwise use 20000 + original port.
func calculateDevPort(currentPort string) string {
	if currentPort != "" {
		// Port was explicitly set in environment, use it
		return currentPort
	}

	// Default to 28080 (20000 + 8080) for development
	return "28080"
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupLogging configures the global slog logger with the specified level.
func setupLogging(level string) {
	var logLevel slog.Level

	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Create a JSON handler for structured logging
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	// Set as default logger
	slog.SetDefault(slog.New(handler))
}
