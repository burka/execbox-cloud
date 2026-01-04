-- Migration: 001_initial
-- Description: Initial schema for execbox-cloud
-- Creates tables for API key management, session tracking, and usage metrics

-- ============================================================================
-- API Keys Table
-- ============================================================================
-- Stores API keys with tier-based configuration and rate limiting
-- Each key is prefixed with 'eb_' for easy identification
COMMENT ON TABLE api_keys IS 'API keys for authentication and authorization with tier-based rate limiting';

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT UNIQUE NOT NULL,
    tier TEXT NOT NULL DEFAULT 'free',
    rate_limit_rps INTEGER DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,

    CONSTRAINT api_keys_tier_check CHECK (tier IN ('free', 'pro', 'enterprise')),
    CONSTRAINT api_keys_rate_limit_check CHECK (rate_limit_rps > 0)
);

-- Index for fast key lookups during authentication
CREATE INDEX idx_api_keys_key ON api_keys(key);

-- Index for querying by tier
CREATE INDEX idx_api_keys_tier ON api_keys(tier);

COMMENT ON COLUMN api_keys.id IS 'Unique identifier for the API key';
COMMENT ON COLUMN api_keys.key IS 'The actual API key (format: eb_xxxxxxxx)';
COMMENT ON COLUMN api_keys.tier IS 'Subscription tier: free, pro, or enterprise';
COMMENT ON COLUMN api_keys.rate_limit_rps IS 'Rate limit in requests per second';
COMMENT ON COLUMN api_keys.created_at IS 'Timestamp when the key was created';
COMMENT ON COLUMN api_keys.last_used_at IS 'Timestamp of last successful authentication';

-- ============================================================================
-- Sessions Table
-- ============================================================================
-- Tracks execution sessions including Fly.io machine details and status
-- Each session represents a containerized execution environment
COMMENT ON TABLE sessions IS 'Execution sessions with Fly.io machine mapping and lifecycle tracking';

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    fly_machine_id TEXT,
    fly_app_id TEXT,
    image TEXT NOT NULL,
    command JSONB,
    env JSONB,
    setup_hash TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    exit_code INTEGER,
    ports JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,

    CONSTRAINT sessions_status_check CHECK (status IN ('pending', 'running', 'stopped', 'failed')),
    CONSTRAINT sessions_exit_code_check CHECK (exit_code IS NULL OR exit_code >= 0)
);

-- Index for querying sessions by API key
CREATE INDEX idx_sessions_api_key_id ON sessions(api_key_id);

-- Index for filtering by status
CREATE INDEX idx_sessions_status ON sessions(status);

-- Composite index for common query pattern: active sessions by API key
CREATE INDEX idx_sessions_api_key_status ON sessions(api_key_id, status);

-- Index for querying by Fly machine ID
CREATE INDEX idx_sessions_fly_machine_id ON sessions(fly_machine_id) WHERE fly_machine_id IS NOT NULL;

COMMENT ON COLUMN sessions.id IS 'Session identifier (format: sess_abc123)';
COMMENT ON COLUMN sessions.api_key_id IS 'Reference to the API key that created this session';
COMMENT ON COLUMN sessions.fly_machine_id IS 'Fly.io machine identifier';
COMMENT ON COLUMN sessions.fly_app_id IS 'Fly.io application identifier';
COMMENT ON COLUMN sessions.image IS 'Docker image used for this session';
COMMENT ON COLUMN sessions.command IS 'Command to execute (JSON array)';
COMMENT ON COLUMN sessions.env IS 'Environment variables (JSON object)';
COMMENT ON COLUMN sessions.setup_hash IS 'Hash of setup configuration for deduplication';
COMMENT ON COLUMN sessions.status IS 'Current status: pending, running, stopped, or failed';
COMMENT ON COLUMN sessions.exit_code IS 'Process exit code (NULL if not yet exited)';
COMMENT ON COLUMN sessions.ports IS 'Port mappings (JSON object)';
COMMENT ON COLUMN sessions.created_at IS 'Timestamp when session was created';
COMMENT ON COLUMN sessions.started_at IS 'Timestamp when session started running';
COMMENT ON COLUMN sessions.ended_at IS 'Timestamp when session ended';

-- ============================================================================
-- Usage Metrics Table
-- ============================================================================
-- Aggregates daily usage statistics per API key for billing and analytics
-- Uses UNIQUE constraint to ensure one row per API key per day
COMMENT ON TABLE usage_metrics IS 'Daily aggregated usage metrics per API key for billing and analytics';

CREATE TABLE usage_metrics (
    id BIGSERIAL PRIMARY KEY,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    executions INTEGER NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT usage_metrics_api_key_date_unique UNIQUE(api_key_id, date),
    CONSTRAINT usage_metrics_executions_check CHECK (executions >= 0),
    CONSTRAINT usage_metrics_duration_check CHECK (duration_ms >= 0)
);

-- Index for efficient daily metrics queries
CREATE INDEX idx_usage_metrics_api_key_date ON usage_metrics(api_key_id, date DESC);

-- Index for querying metrics by date range
CREATE INDEX idx_usage_metrics_date ON usage_metrics(date);

COMMENT ON COLUMN usage_metrics.id IS 'Auto-incrementing unique identifier';
COMMENT ON COLUMN usage_metrics.api_key_id IS 'Reference to the API key';
COMMENT ON COLUMN usage_metrics.date IS 'Date for this metric aggregation';
COMMENT ON COLUMN usage_metrics.executions IS 'Total number of executions on this date';
COMMENT ON COLUMN usage_metrics.duration_ms IS 'Total execution duration in milliseconds';
