package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Client wraps a PostgreSQL connection pool and provides database operations.
type Client struct {
	pool *pgxpool.Pool
}

// New creates a new database client with the provided connection URL.
// The connection pool is configured with sensible defaults and will ping
// the database to verify connectivity before returning.
func New(ctx context.Context, databaseURL string) (*Client, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool with reasonable defaults
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{pool: pool}, nil
}

// Close closes the database connection pool and releases all resources.
func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

// Health checks the database connection health by executing a simple query.
func (c *Client) Health(ctx context.Context) error {
	if c.pool == nil {
		return fmt.Errorf("database client not initialized")
	}

	var result int
	err := c.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("health check returned unexpected value: %d", result)
	}

	return nil
}
