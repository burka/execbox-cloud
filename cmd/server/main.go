package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/burka/execbox-cloud/internal/api"
)

func main() {
	// 1. Load configuration from environment
	cfg := loadConfig()

	// 2. Set up slog with configured log level
	setupLogging(cfg.LogLevel)

	slog.Info("starting execbox-cloud server",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
	)

	// 3. Create server
	server, err := api.NewServer(cfg)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	// 4. Set up HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Handle graceful shutdown
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

// loadConfig reads configuration from environment variables with sensible defaults.
func loadConfig() *api.Config {
	return &api.Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		FlyToken:    getEnv("FLY_API_TOKEN", ""),
		FlyOrg:      getEnv("FLY_ORG", ""),
		FlyAppName:  getEnv("FLY_APP_NAME", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}
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
