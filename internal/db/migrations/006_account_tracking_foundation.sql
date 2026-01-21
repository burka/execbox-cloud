-- Migration: 006_account_tracking_foundation
-- Description: Enhanced account-level tracking foundation that supports future multi-key implementation
-- This migration is backwards compatible and prepares the database for multi-key support

-- Add account_id to existing tables for future aggregation
-- Initially, account_id = id for backwards compatibility (each key is its own account)
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS account_id UUID;

-- Create account-level limits table
CREATE TABLE IF NOT EXISTS account_limits (
    account_id UUID PRIMARY KEY,
    daily_requests_limit INTEGER DEFAULT -1,
    concurrent_requests_limit INTEGER DEFAULT -1,
    monthly_cost_limit DECIMAL(10,2) DEFAULT NULL, -- in USD
    alert_threshold_percentage INTEGER DEFAULT 80,
    billing_email TEXT,
    timezone TEXT DEFAULT 'UTC',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT account_limits_daily_requests_check CHECK (daily_requests_limit = -1 OR daily_requests_limit > 0),
    CONSTRAINT account_limits_concurrent_requests_check CHECK (concurrent_requests_limit = -1 OR concurrent_requests_limit > 0),
    CONSTRAINT account_limits_monthly_cost_check CHECK (monthly_cost_limit IS NULL OR monthly_cost_limit > 0),
    CONSTRAINT account_limits_alert_threshold_check CHECK (alert_threshold_percentage > 0 AND alert_threshold_percentage <= 100)
);

-- Update existing api_keys to have account_id = id (backwards compatible)
UPDATE api_keys SET account_id = id WHERE account_id IS NULL;
ALTER TABLE api_keys ALTER COLUMN account_id SET NOT NULL;

-- Add foreign key constraint after data migration
ALTER TABLE api_keys ADD CONSTRAINT IF NOT EXISTS api_keys_account_id_fkey 
    FOREIGN KEY (account_id) REFERENCES api_keys(id) ON DELETE CASCADE;

-- Create index for account-based queries
CREATE INDEX IF NOT EXISTS idx_api_keys_account_id ON api_keys(account_id);

-- Enhance usage_metrics with account-level tracking
ALTER TABLE usage_metrics ADD COLUMN IF NOT EXISTS account_id UUID;
CREATE INDEX IF NOT EXISTS idx_usage_metrics_account_date ON usage_metrics(account_id, date DESC);

-- Update existing usage_metrics to have account_id = api_key_id
UPDATE usage_metrics SET account_id = api_key_id WHERE account_id IS NULL;
ALTER TABLE usage_metrics ALTER COLUMN account_id SET NOT NULL;

-- Add cost tracking to usage_metrics
ALTER TABLE usage_metrics ADD COLUMN IF NOT EXISTS cost_estimate_cents INTEGER DEFAULT 0;
ALTER TABLE usage_metrics ADD COLUMN IF NOT EXISTS cpu_millis_used BIGINT DEFAULT 0;
ALTER TABLE usage_metrics ADD COLUMN IF NOT EXISTS memory_mb_seconds BIGINT DEFAULT 0; -- MB-seconds for memory usage

-- Add constraints for cost tracking
ALTER TABLE usage_metrics ADD CONSTRAINT IF NOT EXISTS usage_metrics_cost_check 
    CHECK (cost_estimate_cents >= 0);
ALTER TABLE usage_metrics ADD CONSTRAINT IF NOT EXISTS usage_metrics_cpu_check 
    CHECK (cpu_millis_used >= 0);
ALTER TABLE usage_metrics ADD CONSTRAINT IF NOT EXISTS usage_metrics_memory_check 
    CHECK (memory_mb_seconds >= 0);

-- Enhance sessions table for better cost attribution
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS account_id UUID;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cost_estimate_cents INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cpu_millis_used INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS memory_peak_mb INTEGER DEFAULT 0;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS network_transfer_mb INTEGER DEFAULT 0;

-- Update existing sessions to have account_id = api_key_id
UPDATE sessions SET account_id = api_key_id WHERE account_id IS NULL;
ALTER TABLE sessions ALTER COLUMN account_id SET NOT NULL;

-- Add indexes for enhanced session tracking
CREATE INDEX IF NOT EXISTS idx_sessions_account_id ON sessions(account_id);
CREATE INDEX IF NOT EXISTS idx_sessions_account_created ON sessions(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_account_status ON sessions(account_id, status);

-- Create hourly account usage aggregations for better dashboard performance
CREATE TABLE IF NOT EXISTS hourly_account_usage (
    id BIGSERIAL PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    hour TIMESTAMPTZ NOT NULL,
    executions INTEGER NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    cost_estimate_cents BIGINT NOT NULL DEFAULT 0,
    cpu_millis_used BIGINT NOT NULL DEFAULT 0,
    memory_mb_seconds BIGINT NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT hourly_account_usage_account_hour_unique UNIQUE(account_id, hour),
    CONSTRAINT hourly_account_usage_executions_check CHECK (executions >= 0),
    CONSTRAINT hourly_account_usage_cost_check CHECK (cost_estimate_cents >= 0),
    CONSTRAINT hourly_account_usage_cpu_check CHECK (cpu_millis_used >= 0),
    CONSTRAINT hourly_account_usage_memory_check CHECK (memory_mb_seconds >= 0),
    CONSTRAINT hourly_account_usage_errors_check CHECK (errors >= 0)
);

-- Create indexes for hourly account usage
CREATE INDEX IF NOT EXISTS idx_hourly_account_usage_account_hour ON hourly_account_usage(account_id, hour DESC);
CREATE INDEX IF NOT EXISTS idx_hourly_account_usage_hour ON hourly_account_usage(hour);

-- Usage attribution table (prepares for multi-key but works with single key)
CREATE TABLE IF NOT EXISTS usage_attribution (
    id BIGSERIAL PRIMARY KEY,
    usage_metric_id INTEGER REFERENCES usage_metrics(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    account_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    project_tag TEXT, -- future: project/team/environment tagging
    environment_tag TEXT,
    cost_center_tag TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT usage_attribution_unique UNIQUE(usage_metric_id)
);

-- Indexes for usage attribution
CREATE INDEX IF NOT EXISTS idx_usage_attribution_account_id ON usage_attribution(account_id);
CREATE INDEX IF NOT EXISTS idx_usage_attribution_api_key_id ON usage_attribution(api_key_id);
CREATE INDEX IF NOT EXISTS idx_usage_attribution_project_tag ON usage_attribution(project_tag) WHERE project_tag IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_usage_attribution_environment_tag ON usage_attribution(environment_tag) WHERE environment_tag IS NOT NULL;

-- Account cost tracking table
CREATE TABLE IF NOT EXISTS account_cost_tracking (
    id BIGSERIAL PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    daily_cost_cents INTEGER NOT NULL DEFAULT 0,
    daily_executions INTEGER NOT NULL DEFAULT 0,
    billing_period_start DATE NOT NULL,
    billing_period_end DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT account_cost_tracking_account_date_unique UNIQUE(account_id, date),
    CONSTRAINT account_cost_tracking_cost_check CHECK (daily_cost_cents >= 0),
    CONSTRAINT account_cost_tracking_executions_check CHECK (daily_executions >= 0)
);

-- Indexes for cost tracking
CREATE INDEX IF NOT EXISTS idx_account_cost_tracking_account_date ON account_cost_tracking(account_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_account_cost_tracking_billing_period ON account_cost_tracking(billing_period_start, account_id);

-- Function to update hourly account usage
CREATE OR REPLACE FUNCTION update_hourly_account_usage()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO hourly_account_usage (
        account_id, 
        hour, 
        executions, 
        duration_ms, 
        cost_estimate_cents,
        cpu_millis_used,
        memory_mb_seconds,
        errors,
        updated_at
    ) VALUES (
        NEW.account_id,
        date_trunc('hour', NEW.created_at),
        1,
        CASE WHEN NEW.started_at IS NOT NULL AND NEW.ended_at IS NOT NULL 
             THEN EXTRACT(EPOCH FROM (NEW.ended_at - NEW.started_at)) * 1000 
             ELSE 0 END,
        COALESCE(NEW.cost_estimate_cents, 0),
        COALESCE(NEW.cpu_millis_used, 0),
        COALESCE(NEW.memory_peak_mb, 0) * CASE WHEN NEW.started_at IS NOT NULL AND NEW.ended_at IS NOT NULL 
                                               THEN EXTRACT(EPOCH FROM (NEW.ended_at - NEW.started_at)) 
                                               ELSE 0 END,
        CASE WHEN NEW.exit_code IS NOT NULL AND NEW.exit_code != 0 THEN 1 ELSE 0 END,
        NOW()
    )
    ON CONFLICT (account_id, hour) DO UPDATE SET
        executions = hourly_account_usage.executions + 1,
        duration_ms = hourly_account_usage.duration_ms + CASE WHEN NEW.started_at IS NOT NULL AND NEW.ended_at IS NOT NULL 
                                                               THEN EXTRACT(EPOCH FROM (NEW.ended_at - NEW.started_at)) * 1000 
                                                               ELSE 0 END,
        cost_estimate_cents = hourly_account_usage.cost_estimate_cents + COALESCE(NEW.cost_estimate_cents, 0),
        cpu_millis_used = hourly_account_usage.cpu_millis_used + COALESCE(NEW.cpu_millis_used, 0),
        memory_mb_seconds = hourly_account_usage.memory_mb_seconds + 
                           (COALESCE(NEW.memory_peak_mb, 0) * CASE WHEN NEW.started_at IS NOT NULL AND NEW.ended_at IS NOT NULL 
                                                                  THEN EXTRACT(EPOCH FROM (NEW.ended_at - NEW.started_at)) 
                                                                  ELSE 0 END),
        errors = hourly_account_usage.errors + CASE WHEN NEW.exit_code IS NOT NULL AND NEW.exit_code != 0 THEN 1 ELSE 0 END,
        updated_at = NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to update daily cost tracking
CREATE OR REPLACE FUNCTION update_account_cost_tracking()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO account_cost_tracking (
        account_id,
        date,
        daily_cost_cents,
        daily_executions,
        billing_period_start,
        billing_period_end,
        updated_at
    ) VALUES (
        NEW.account_id,
        DATE(NEW.created_at),
        COALESCE(NEW.cost_estimate_cents, 0),
        1,
        DATE_TRUNC('month', NEW.created_at)::DATE,
        (DATE_TRUNC('month', NEW.created_at) + INTERVAL '1 month' - INTERVAL '1 day')::DATE,
        NOW()
    )
    ON CONFLICT (account_id, date) DO UPDATE SET
        daily_cost_cents = account_cost_tracking.daily_cost_cents + COALESCE(NEW.cost_estimate_cents, 0),
        daily_executions = account_cost_tracking.daily_executions + 1,
        updated_at = NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to create usage attribution when usage_metrics is updated
CREATE OR REPLACE FUNCTION create_usage_attribution()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO usage_attribution (usage_metric_id, session_id, account_id, api_key_id)
    VALUES (NEW.id, 
            (SELECT id FROM sessions WHERE api_key_id = NEW.api_key_id ORDER BY created_at DESC LIMIT 1),
            NEW.account_id,
            NEW.api_key_id)
    ON CONFLICT (usage_metric_id) DO NOTHING;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers
CREATE TRIGGER trigger_update_hourly_account_usage
    AFTER INSERT OR UPDATE ON sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_hourly_account_usage();

CREATE TRIGGER trigger_update_account_cost_tracking
    AFTER INSERT ON sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_account_cost_tracking();

CREATE TRIGGER trigger_create_usage_attribution
    AFTER INSERT OR UPDATE ON usage_metrics
    FOR EACH ROW
    EXECUTE FUNCTION create_usage_attribution();

-- Create default account limits for existing accounts
INSERT INTO account_limits (account_id, daily_requests_limit, concurrent_requests_limit)
SELECT id, 
       CASE tier 
           WHEN 'free' THEN 10
           WHEN 'starter' THEN 100
           WHEN 'pro' THEN 1000
           WHEN 'enterprise' THEN -1
           ELSE 10
       END,
       CASE tier
           WHEN 'free' THEN 5
           WHEN 'starter' THEN 10
           WHEN 'pro' THEN 50
           WHEN 'enterprise' THEN -1
           ELSE 5
       END
FROM api_keys 
WHERE id NOT IN (SELECT account_id FROM account_limits);

-- Comments
COMMENT ON COLUMN api_keys.account_id IS 'Account identifier for billing and usage aggregation (initially self-referencing for backwards compatibility)';
COMMENT ON TABLE account_limits IS 'Account-specific limits and billing configuration';
COMMENT ON COLUMN usage_metrics.account_id IS 'Account identifier for aggregated usage tracking';
COMMENT ON COLUMN usage_metrics.cost_estimate_cents IS 'Cost estimate in cents for billing calculations';
COMMENT ON COLUMN usage_metrics.cpu_millis_used IS 'Total CPU time in milliseconds';
COMMENT ON COLUMN usage_metrics.memory_mb_seconds IS 'Memory usage in MB-seconds';
COMMENT ON TABLE hourly_account_usage IS 'Hourly aggregated usage data for dashboard performance';
COMMENT ON TABLE usage_attribution IS 'Usage attribution for future multi-key and project-based tracking';
COMMENT ON TABLE account_cost_tracking IS 'Daily cost tracking per account for billing and budget management';