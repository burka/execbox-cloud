package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetAPIKeyByKey retrieves an API key by its key string.
// Returns an error if the key is not found or if the query fails.
func (c *Client) GetAPIKeyByKey(ctx context.Context, key string) (*APIKey, error) {
	query := `
		SELECT id, key, email, tier, tier_expires_at, tier_updated_at, rate_limit_rps, created_at, last_used_at
		FROM api_keys
		WHERE key = $1
	`

	var apiKey APIKey
	err := c.pool.QueryRow(ctx, query, key).Scan(
		&apiKey.ID,
		&apiKey.Key,
		&apiKey.Email,
		&apiKey.Tier,
		&apiKey.TierExpiresAt,
		&apiKey.TierUpdatedAt,
		&apiKey.RateLimitRPS,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return &apiKey, nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key.
func (c *Client) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE api_keys
		SET last_used_at = NOW()
		WHERE id = $1
	`

	result, err := c.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// CreateSession creates a new session in the database.
func (c *Client) CreateSession(ctx context.Context, sess *Session) error {
	// Marshal JSON fields
	commandJSON, err := json.Marshal(sess.Command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	envJSON, err := json.Marshal(sess.Env)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	portsJSON, err := json.Marshal(sess.Ports)
	if err != nil {
		return fmt.Errorf("failed to marshal ports: %w", err)
	}

	query := `
		INSERT INTO sessions (
			id, api_key_id, fly_machine_id, fly_app_id, image, command, env,
			setup_hash, status, exit_code, ports, created_at, started_at, ended_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = c.pool.Exec(ctx, query,
		sess.ID,
		sess.APIKeyID,
		sess.FlyMachineID,
		sess.FlyAppID,
		sess.Image,
		commandJSON,
		envJSON,
		sess.SetupHash,
		sess.Status,
		sess.ExitCode,
		portsJSON,
		sess.CreatedAt,
		sess.StartedAt,
		sess.EndedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by its ID.
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	query := `
		SELECT id, api_key_id, fly_machine_id, fly_app_id, image, command, env,
		       setup_hash, status, exit_code, ports, created_at, started_at, ended_at
		FROM sessions
		WHERE id = $1
	`

	var sess Session
	var commandJSON, envJSON, portsJSON []byte

	err := c.pool.QueryRow(ctx, query, id).Scan(
		&sess.ID,
		&sess.APIKeyID,
		&sess.FlyMachineID,
		&sess.FlyAppID,
		&sess.Image,
		&commandJSON,
		&envJSON,
		&sess.SetupHash,
		&sess.Status,
		&sess.ExitCode,
		&portsJSON,
		&sess.CreatedAt,
		&sess.StartedAt,
		&sess.EndedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal JSON fields
	if commandJSON != nil {
		if err := json.Unmarshal(commandJSON, &sess.Command); err != nil {
			return nil, fmt.Errorf("failed to unmarshal command: %w", err)
		}
	}

	if envJSON != nil {
		if err := json.Unmarshal(envJSON, &sess.Env); err != nil {
			return nil, fmt.Errorf("failed to unmarshal env: %w", err)
		}
	}

	if portsJSON != nil {
		if err := json.Unmarshal(portsJSON, &sess.Ports); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ports: %w", err)
		}
	}

	return &sess, nil
}

// ListSessions retrieves sessions for an API key, optionally filtered by status.
func (c *Client) ListSessions(ctx context.Context, apiKeyID uuid.UUID, status *string) ([]Session, error) {
	var query string
	var args []interface{}

	if status != nil {
		query = `
			SELECT id, api_key_id, fly_machine_id, fly_app_id, image, command, env,
			       setup_hash, status, exit_code, ports, created_at, started_at, ended_at
			FROM sessions
			WHERE api_key_id = $1 AND status = $2
			ORDER BY created_at DESC
		`
		args = []interface{}{apiKeyID, *status}
	} else {
		query = `
			SELECT id, api_key_id, fly_machine_id, fly_app_id, image, command, env,
			       setup_hash, status, exit_code, ports, created_at, started_at, ended_at
			FROM sessions
			WHERE api_key_id = $1
			ORDER BY created_at DESC
		`
		args = []interface{}{apiKeyID}
	}

	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var commandJSON, envJSON, portsJSON []byte

		err := rows.Scan(
			&sess.ID,
			&sess.APIKeyID,
			&sess.FlyMachineID,
			&sess.FlyAppID,
			&sess.Image,
			&commandJSON,
			&envJSON,
			&sess.SetupHash,
			&sess.Status,
			&sess.ExitCode,
			&portsJSON,
			&sess.CreatedAt,
			&sess.StartedAt,
			&sess.EndedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}

		// Unmarshal JSON fields
		if commandJSON != nil {
			if err := json.Unmarshal(commandJSON, &sess.Command); err != nil {
				return nil, fmt.Errorf("failed to unmarshal command: %w", err)
			}
		}

		if envJSON != nil {
			if err := json.Unmarshal(envJSON, &sess.Env); err != nil {
				return nil, fmt.Errorf("failed to unmarshal env: %w", err)
			}
		}

		if portsJSON != nil {
			if err := json.Unmarshal(portsJSON, &sess.Ports); err != nil {
				return nil, fmt.Errorf("failed to unmarshal ports: %w", err)
			}
		}

		sessions = append(sessions, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// UpdateSession updates a session with the provided fields.
func (c *Client) UpdateSession(ctx context.Context, id string, update *SessionUpdate) error {
	// Build dynamic update query based on provided fields
	query := "UPDATE sessions SET"
	args := []interface{}{}
	argPos := 1
	updates := []string{}

	if update.FlyMachineID != nil {
		updates = append(updates, fmt.Sprintf(" fly_machine_id = $%d", argPos))
		args = append(args, *update.FlyMachineID)
		argPos++
	}

	if update.FlyAppID != nil {
		updates = append(updates, fmt.Sprintf(" fly_app_id = $%d", argPos))
		args = append(args, *update.FlyAppID)
		argPos++
	}

	if update.Status != nil {
		updates = append(updates, fmt.Sprintf(" status = $%d", argPos))
		args = append(args, *update.Status)
		argPos++
	}

	if update.ExitCode != nil {
		updates = append(updates, fmt.Sprintf(" exit_code = $%d", argPos))
		args = append(args, *update.ExitCode)
		argPos++
	}

	if update.Ports != nil {
		portsJSON, err := json.Marshal(update.Ports)
		if err != nil {
			return fmt.Errorf("failed to marshal ports: %w", err)
		}
		updates = append(updates, fmt.Sprintf(" ports = $%d", argPos))
		args = append(args, portsJSON)
		argPos++
	}

	if update.StartedAt != nil {
		updates = append(updates, fmt.Sprintf(" started_at = $%d", argPos))
		args = append(args, *update.StartedAt)
		argPos++
	}

	if update.EndedAt != nil {
		updates = append(updates, fmt.Sprintf(" ended_at = $%d", argPos))
		args = append(args, *update.EndedAt)
		argPos++
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Complete the query
	for i, u := range updates {
		if i > 0 {
			query += ","
		}
		query += u
	}
	query += fmt.Sprintf(" WHERE id = $%d", argPos)
	args = append(args, id)

	result, err := c.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// DeleteSession deletes a session by its ID.
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	query := "DELETE FROM sessions WHERE id = $1"

	result, err := c.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// IncrementUsage increments usage metrics for an API key for today's date.
// This uses an INSERT ... ON CONFLICT to atomically update or create the daily metric.
func (c *Client) IncrementUsage(ctx context.Context, apiKeyID uuid.UUID, durationMs int64) error {
	query := `
		INSERT INTO usage_metrics (api_key_id, date, executions, duration_ms)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (api_key_id, date)
		DO UPDATE SET
			executions = usage_metrics.executions + 1,
			duration_ms = usage_metrics.duration_ms + $3
	`

	today := time.Now().UTC().Truncate(24 * time.Hour)

	_, err := c.pool.Exec(ctx, query, apiKeyID, today, durationMs)
	if err != nil {
		return fmt.Errorf("failed to increment usage: %w", err)
	}

	return nil
}

// ============================================================================
// Image Cache Queries
// ============================================================================

// GetImageCache retrieves a cached image by its hash.
// Returns (registryTag, true, nil) if found, ("", false, nil) if not found.
func (c *Client) GetImageCache(ctx context.Context, hash string) (string, bool, error) {
	query := `
		SELECT registry_tag
		FROM image_cache
		WHERE hash = $1
	`

	var registryTag string
	err := c.pool.QueryRow(ctx, query, hash).Scan(&registryTag)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to get image cache: %w", err)
	}

	return registryTag, true, nil
}

// PutImageCache stores a new image cache entry.
func (c *Client) PutImageCache(ctx context.Context, hash, baseImage, registryTag string) error {
	query := `
		INSERT INTO image_cache (hash, base_image, registry_tag, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (hash) DO NOTHING
	`

	_, err := c.pool.Exec(ctx, query, hash, baseImage, registryTag)
	if err != nil {
		return fmt.Errorf("failed to put image cache: %w", err)
	}

	return nil
}

// TouchImageCache updates the last_used_at timestamp for a cached image.
func (c *Client) TouchImageCache(ctx context.Context, hash string) error {
	query := `
		UPDATE image_cache
		SET last_used_at = NOW()
		WHERE hash = $1
	`

	_, err := c.pool.Exec(ctx, query, hash)
	if err != nil {
		return fmt.Errorf("failed to touch image cache: %w", err)
	}

	return nil
}

// ============================================================================
// Quota Enforcement Queries
// ============================================================================

// GetActiveSessionCount counts sessions with status 'running' or 'pending' for an API key.
func (c *Client) GetActiveSessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM sessions
		WHERE api_key_id = $1
		  AND status IN ('running', 'pending')
	`

	var count int
	err := c.pool.QueryRow(ctx, query, apiKeyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active session count: %w", err)
	}

	return count, nil
}

// GetDailySessionCount counts sessions created today (UTC) for an API key.
func (c *Client) GetDailySessionCount(ctx context.Context, apiKeyID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM sessions
		WHERE api_key_id = $1
		  AND created_at >= DATE_TRUNC('day', NOW() AT TIME ZONE 'UTC')
	`

	var count int
	err := c.pool.QueryRow(ctx, query, apiKeyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get daily session count: %w", err)
	}

	return count, nil
}

// ============================================================================
// Quota Request Queries
// ============================================================================

// CreateQuotaRequest creates a new quota request and returns the created record.
func (c *Client) CreateQuotaRequest(ctx context.Context, req *QuotaRequest) (*QuotaRequest, error) {
	query := `
		INSERT INTO quota_requests (
			api_key_id, email, name, company, current_tier,
			requested_limits, budget, use_case, status, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', NOW())
		RETURNING id, created_at
	`

	err := c.pool.QueryRow(ctx, query,
		req.APIKeyID,
		req.Email,
		req.Name,
		req.Company,
		req.CurrentTier,
		req.RequestedLimits,
		req.Budget,
		req.UseCase,
	).Scan(&req.ID, &req.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create quota request: %w", err)
	}

	req.Status = "pending"
	return req, nil
}
