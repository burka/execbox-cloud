package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// apiKeyColumns is the list of columns to select for API key queries
const apiKeyColumns = `id, key, email, tier, tier_expires_at, tier_updated_at, rate_limit_rps,
    created_at, last_used_at, name, description, is_active, expires_at,
    parent_key_id, account_id, custom_daily_limit, custom_concurrent_limit,
    last_updated_by, metadata`

// scanAPIKey scans a database row into an APIKey struct
func scanAPIKey(row interface{ Scan(...any) error }) (*APIKey, error) {
	var key APIKey
	err := row.Scan(
		&key.ID, &key.Key, &key.Email, &key.Tier, &key.TierExpiresAt,
		&key.TierUpdatedAt, &key.RateLimitRPS, &key.CreatedAt, &key.LastUsedAt,
		&key.Name, &key.Description, &key.IsActive, &key.ExpiresAt,
		&key.ParentKeyID, &key.AccountID, &key.CustomDailyLimit,
		&key.CustomConcurrentLimit, &key.LastUpdatedBy, &key.Metadata,
	)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// GetAPIKeyByKey retrieves an API key by its key string.
// Returns an error if the key is not found, deactivated, expired, or if the query fails.
func (c *Client) GetAPIKeyByKey(ctx context.Context, key string) (*APIKey, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM api_keys
		WHERE key = $1
		  AND is_active = true
		  AND (expires_at IS NULL OR expires_at > NOW())
	`, apiKeyColumns)

	apiKey, err := scanAPIKey(c.pool.QueryRow(ctx, query, key))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return apiKey, nil
}

// GetAPIKeyByID retrieves an API key by its ID.
// Returns the key regardless of active/expired status (for management operations).
func (c *Client) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM api_keys
		WHERE id = $1
	`, apiKeyColumns)

	apiKey, err := scanAPIKey(c.pool.QueryRow(ctx, query, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return apiKey, nil
}

// CreateAPIKey creates a new API key for the given email with free tier.
// Returns the created API key with a newly generated key string.
// This creates the primary key for a new account (account_id = key id).
func (c *Client) CreateAPIKey(ctx context.Context, email string, name *string) (*APIKey, error) {
	// Generate a secure random API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}
	key := fmt.Sprintf("sk_%s", hex.EncodeToString(keyBytes))

	// For primary keys, account_id equals the key's own ID
	keyID := uuid.New()

	query := fmt.Sprintf(`
		INSERT INTO api_keys (id, key, email, tier, rate_limit_rps, is_active, account_id, created_at)
		VALUES ($1, $2, $3, 'free', 10, true, $1, NOW())
		RETURNING %s
	`, apiKeyColumns)

	apiKey, err := scanAPIKey(c.pool.QueryRow(ctx, query, keyID, key, email))
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	// Create default account limits for the new account
	limitsQuery := `
		INSERT INTO account_limits (account_id, daily_requests_limit, concurrent_requests_limit, alert_threshold_percentage, timezone, created_at, updated_at)
		VALUES ($1, 100, 5, 80, 'UTC', NOW(), NOW())
	`
	_, limitsErr := c.pool.Exec(ctx, limitsQuery, apiKey.ID)
	if limitsErr != nil {
		// Log the error but don't fail the API key creation
		// GetAccountLimits has a fallback, so this is not critical
		fmt.Printf("Warning: failed to create default account limits for %s: %v\n", apiKey.ID, limitsErr)
	}

	return apiKey, nil
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
			id, api_key_id, account_id, fly_machine_id, fly_app_id, image, command, env,
			setup_hash, status, exit_code, ports, created_at, started_at, ended_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err = c.pool.Exec(ctx, query,
		sess.ID,
		sess.APIKeyID,
		sess.AccountID,
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
		SELECT id, api_key_id, account_id, fly_machine_id, fly_app_id, image, command, env,
		       setup_hash, status, exit_code, ports, created_at, started_at, ended_at
		FROM sessions
		WHERE id = $1
	`

	var sess Session
	var commandJSON, envJSON, portsJSON []byte

	err := c.pool.QueryRow(ctx, query, id).Scan(
		&sess.ID,
		&sess.APIKeyID,
		&sess.AccountID,
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
			SELECT id, api_key_id, account_id, fly_machine_id, fly_app_id, image, command, env,
			       setup_hash, status, exit_code, ports, created_at, started_at, ended_at
			FROM sessions
			WHERE api_key_id = $1 AND status = $2
			ORDER BY created_at DESC
		`
		args = []interface{}{apiKeyID, *status}
	} else {
		query = `
			SELECT id, api_key_id, account_id, fly_machine_id, fly_app_id, image, command, env,
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
			&sess.AccountID,
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

	if update.CostEstimateCents != nil {
		updates = append(updates, fmt.Sprintf(" cost_estimate_cents = $%d", argPos))
		args = append(args, *update.CostEstimateCents)
		argPos++
	}

	if update.CPUMillisUsed != nil {
		updates = append(updates, fmt.Sprintf(" cpu_millis_used = $%d", argPos))
		args = append(args, *update.CPUMillisUsed)
		argPos++
	}

	if update.MemoryPeakMB != nil {
		updates = append(updates, fmt.Sprintf(" memory_peak_mb = $%d", argPos))
		args = append(args, *update.MemoryPeakMB)
		argPos++
	}

	if update.DurationMs != nil {
		updates = append(updates, fmt.Sprintf(" duration_ms = $%d", argPos))
		args = append(args, *update.DurationMs)
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

// ============================================================================
// Account Usage Queries
// ============================================================================

// GetAccountLimits retrieves account limits by account ID.
// Returns an error if the limits are not found or if the query fails.
func (c *Client) GetAccountLimits(ctx context.Context, accountID uuid.UUID) (*AccountLimits, error) {
	query := `
		SELECT account_id, daily_requests_limit, concurrent_requests_limit,
		       CASE WHEN monthly_cost_limit IS NOT NULL THEN (monthly_cost_limit * 100)::bigint ELSE NULL END,
		       alert_threshold_percentage, billing_email, timezone, updated_at, created_at
		FROM account_limits
		WHERE account_id = $1
	`

	var limits AccountLimits
	err := c.pool.QueryRow(ctx, query, accountID).Scan(
		&limits.AccountID,
		&limits.DailyRequestsLimit,
		&limits.ConcurrentRequestsLimit,
		&limits.MonthlyCostLimitCents,
		&limits.AlertThresholdPercentage,
		&limits.BillingEmail,
		&limits.Timezone,
		&limits.UpdatedAt,
		&limits.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("account limits not found")
		}
		return nil, fmt.Errorf("failed to get account limits: %w", err)
	}

	return &limits, nil
}

// UpsertAccountLimits creates or updates account limits.
// Uses INSERT ... ON CONFLICT to atomically create or update the record.
func (c *Client) UpsertAccountLimits(ctx context.Context, limits *AccountLimits) error {
	query := `
		INSERT INTO account_limits (
			account_id, daily_requests_limit, concurrent_requests_limit, monthly_cost_limit,
			alert_threshold_percentage, billing_email, timezone, created_at, updated_at
		)
		VALUES ($1, $2, $3, CASE WHEN $4::bigint IS NOT NULL THEN $4::numeric / 100 ELSE NULL END, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (account_id) DO UPDATE SET
			daily_requests_limit = EXCLUDED.daily_requests_limit,
			concurrent_requests_limit = EXCLUDED.concurrent_requests_limit,
			monthly_cost_limit = EXCLUDED.monthly_cost_limit,
			alert_threshold_percentage = EXCLUDED.alert_threshold_percentage,
			billing_email = EXCLUDED.billing_email,
			timezone = EXCLUDED.timezone,
			updated_at = NOW()
	`

	_, err := c.pool.Exec(ctx, query,
		limits.AccountID,
		limits.DailyRequestsLimit,
		limits.ConcurrentRequestsLimit,
		limits.MonthlyCostLimitCents,
		limits.AlertThresholdPercentage,
		limits.BillingEmail,
		limits.Timezone,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert account limits: %w", err)
	}

	return nil
}

// GetHourlyAccountUsage retrieves hourly usage metrics for an account within a time range.
// Returns an empty slice if no records are found.
func (c *Client) GetHourlyAccountUsage(ctx context.Context, accountID uuid.UUID, start, end time.Time) ([]HourlyAccountUsage, error) {
	query := `
		SELECT id, account_id, hour, executions, duration_ms, cost_estimate_cents, cpu_millis_used, memory_mb_seconds, errors, created_at, updated_at
		FROM hourly_account_usage
		WHERE account_id = $1
		  AND hour BETWEEN $2 AND $3
		ORDER BY hour ASC
	`

	rows, err := c.pool.Query(ctx, query, accountID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get hourly account usage: %w", err)
	}
	defer rows.Close()

	var usage []HourlyAccountUsage
	for rows.Next() {
		var u HourlyAccountUsage
		err := rows.Scan(
			&u.ID,
			&u.AccountID,
			&u.Hour,
			&u.Executions,
			&u.DurationMs,
			&u.CostEstimateCents,
			&u.CPUMillisUsed,
			&u.MemoryMBSeconds,
			&u.Errors,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hourly account usage row: %w", err)
		}
		usage = append(usage, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating hourly account usage: %w", err)
	}

	return usage, nil
}

// GetDailyAccountUsage retrieves daily aggregated usage metrics for an account.
// Returns an empty slice if no records are found.
func (c *Client) GetDailyAccountUsage(ctx context.Context, accountID uuid.UUID, days int) ([]UsageMetric, error) {
	query := `
		SELECT id, api_key_id, date, executions, duration_ms
		FROM usage_metrics
		WHERE account_id = $1
		  AND date >= CURRENT_DATE - $2 * INTERVAL '1 day'
		ORDER BY date DESC
	`

	rows, err := c.pool.Query(ctx, query, accountID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily account usage: %w", err)
	}
	defer rows.Close()

	var usage []UsageMetric
	for rows.Next() {
		var u UsageMetric
		err := rows.Scan(
			&u.ID,
			&u.APIKeyID,
			&u.Date,
			&u.Executions,
			&u.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily account usage row: %w", err)
		}
		usage = append(usage, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily account usage: %w", err)
	}

	return usage, nil
}

// GetAccountCostTracking retrieves cost tracking data for an account for a specific billing period.
// Returns an empty slice if no records are found.
func (c *Client) GetAccountCostTracking(ctx context.Context, accountID uuid.UUID, periodStart time.Time) ([]AccountCostTracking, error) {
	query := `
		SELECT id, account_id, date, daily_cost_cents, daily_executions, billing_period_start, billing_period_end, created_at, updated_at
		FROM account_cost_tracking
		WHERE account_id = $1
		  AND billing_period_start = $2
		ORDER BY date ASC
	`

	rows, err := c.pool.Query(ctx, query, accountID, periodStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get account cost tracking: %w", err)
	}
	defer rows.Close()

	var tracking []AccountCostTracking
	for rows.Next() {
		var t AccountCostTracking
		err := rows.Scan(
			&t.ID,
			&t.AccountID,
			&t.Date,
			&t.DailyCostCents,
			&t.DailyExecutions,
			&t.BillingPeriodStart,
			&t.BillingPeriodEnd,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account cost tracking row: %w", err)
		}
		tracking = append(tracking, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating account cost tracking: %w", err)
	}

	return tracking, nil
}

// ============================================================================
// Multi-Key Management Queries
// ============================================================================

// GetAPIKeysByAccount retrieves all API keys owned by an account.
// Returns all keys including deactivated/expired ones (for management display).
func (c *Client) GetAPIKeysByAccount(ctx context.Context, accountID uuid.UUID) ([]APIKey, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM api_keys
		WHERE account_id = $1
		ORDER BY created_at ASC
	`, apiKeyColumns)

	rows, err := c.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API keys for account: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key row: %w", err)
		}
		keys = append(keys, *k)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API keys: %w", err)
	}

	return keys, nil
}

// CreateAPIKeyForAccount creates a new API key for an existing account.
// The new key inherits tier settings from the parent key.
func (c *Client) CreateAPIKeyForAccount(ctx context.Context, accountID uuid.UUID, name, description string, parentKeyID uuid.UUID) (*APIKey, error) {
	// Generate a secure random API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}
	key := fmt.Sprintf("sk_%s", hex.EncodeToString(keyBytes))

	query := fmt.Sprintf(`
		INSERT INTO api_keys (id, key, email, tier, rate_limit_rps, is_active, account_id, parent_key_id, name, description, created_at)
		SELECT $1, $2, email, tier, rate_limit_rps, true, $3, $4, $5, $6, NOW()
		FROM api_keys
		WHERE id = $4
		RETURNING %s
	`, apiKeyColumns)

	apiKey, err := scanAPIKey(c.pool.QueryRow(ctx, query, uuid.New(), key, accountID, parentKeyID, name, description))
	if err != nil {
		return nil, fmt.Errorf("failed to create API key for account: %w", err)
	}

	return apiKey, nil
}

// UpdateAPIKey updates an API key's fields.
// Only non-nil fields in the update struct will be changed.
func (c *Client) UpdateAPIKey(ctx context.Context, keyID uuid.UUID, update *APIKeyUpdate) error {
	query := "UPDATE api_keys SET"
	args := []interface{}{}
	argPos := 1
	updates := []string{}

	if update.Name != nil {
		updates = append(updates, fmt.Sprintf(" name = $%d", argPos))
		args = append(args, *update.Name)
		argPos++
	}

	if update.Description != nil {
		updates = append(updates, fmt.Sprintf(" description = $%d", argPos))
		args = append(args, *update.Description)
		argPos++
	}

	if update.IsActive != nil {
		updates = append(updates, fmt.Sprintf(" is_active = $%d", argPos))
		args = append(args, *update.IsActive)
		argPos++
	}

	if update.ExpiresAt != nil {
		updates = append(updates, fmt.Sprintf(" expires_at = $%d", argPos))
		args = append(args, *update.ExpiresAt)
		argPos++
	}

	if update.CustomDailyLimit != nil {
		updates = append(updates, fmt.Sprintf(" custom_daily_limit = $%d", argPos))
		args = append(args, *update.CustomDailyLimit)
		argPos++
	}

	if update.CustomConcurrentLimit != nil {
		updates = append(updates, fmt.Sprintf(" custom_concurrent_limit = $%d", argPos))
		args = append(args, *update.CustomConcurrentLimit)
		argPos++
	}

	if update.LastUpdatedBy != nil {
		updates = append(updates, fmt.Sprintf(" last_updated_by = $%d", argPos))
		args = append(args, *update.LastUpdatedBy)
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
	args = append(args, keyID)

	result, err := c.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// DeactivateAPIKey soft-deletes an API key by setting is_active=false.
// Also records who performed the action in last_updated_by.
func (c *Client) DeactivateAPIKey(ctx context.Context, keyID uuid.UUID, performedBy string) error {
	query := `
		UPDATE api_keys
		SET is_active = false, last_updated_by = $2
		WHERE id = $1
	`

	result, err := c.pool.Exec(ctx, query, keyID, performedBy)
	if err != nil {
		return fmt.Errorf("failed to deactivate API key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// RotateAPIKey creates a new key value while preserving the key's settings.
// Returns the updated key with the new key value.
func (c *Client) RotateAPIKey(ctx context.Context, keyID uuid.UUID, performedBy string) (*APIKey, error) {
	// Generate a new secure random API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}
	newKey := fmt.Sprintf("sk_%s", hex.EncodeToString(keyBytes))

	query := fmt.Sprintf(`
		UPDATE api_keys
		SET key = $2, last_updated_by = $3
		WHERE id = $1
		RETURNING %s
	`, apiKeyColumns)

	apiKey, err := scanAPIKey(c.pool.QueryRow(ctx, query, keyID, newKey, performedBy))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to rotate API key: %w", err)
	}

	return apiKey, nil
}

// IsPrimaryKey checks if the given key is the primary key for its account.
// Primary keys have account_id equal to their own ID (or no parent_key_id).
func (c *Client) IsPrimaryKey(ctx context.Context, keyID uuid.UUID) (bool, error) {
	query := `
		SELECT account_id = id OR parent_key_id IS NULL
		FROM api_keys
		WHERE id = $1
	`

	var isPrimary bool
	err := c.pool.QueryRow(ctx, query, keyID).Scan(&isPrimary)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, fmt.Errorf("API key not found")
		}
		return false, fmt.Errorf("failed to check if primary key: %w", err)
	}

	return isPrimary, nil
}
