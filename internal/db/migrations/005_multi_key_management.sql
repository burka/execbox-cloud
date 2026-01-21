-- Migration: 005_multi_key_management
-- Description: Add support for multiple API keys per account with enhanced management

-- Add key management fields to api_keys table
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS parent_key_id UUID REFERENCES api_keys(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS custom_daily_limit INTEGER;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS custom_concurrent_limit INTEGER;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS last_updated_by TEXT; -- Who made the last change
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS metadata JSONB; -- Flexible storage for custom fields

-- Add constraints
ALTER TABLE api_keys ADD CONSTRAINT IF NOT EXISTS api_keys_custom_daily_limit_check 
    CHECK (custom_daily_limit IS NULL OR custom_daily_limit > 0);
ALTER TABLE api_keys ADD CONSTRAINT IF NOT EXISTS api_keys_custom_concurrent_limit_check 
    CHECK (custom_concurrent_limit IS NULL OR custom_concurrent_limit > 0);

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_api_keys_parent_id ON api_keys(parent_key_id) WHERE parent_key_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_name ON api_keys(name) WHERE name IS NOT NULL;

-- Add detailed usage tracking table
CREATE TABLE IF NOT EXISTS detailed_usage (
    id BIGSERIAL PRIMARY KEY,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_ms INTEGER NOT NULL,
    memory_peak_mb INTEGER,
    cpu_millis INTEGER,
    exit_code INTEGER,
    error_message TEXT,
    image TEXT NOT NULL,
    command JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT detailed_usage_duration_check CHECK (duration_ms >= 0),
    CONSTRAINT detailed_usage_memory_check CHECK (memory_peak_mb IS NULL OR memory_peak_mb > 0),
    CONSTRAINT detailed_usage_cpu_check CHECK (cpu_millis IS NULL OR cpu_millis >= 0),
    CONSTRAINT detailed_usage_exit_code_check CHECK (exit_code IS NULL OR exit_code >= 0)
);

-- Indexes for detailed usage queries
CREATE INDEX IF NOT EXISTS idx_detailed_usage_api_key_timestamp ON detailed_usage(api_key_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_detailed_usage_session_id ON detailed_usage(session_id);
CREATE INDEX IF NOT EXISTS idx_detailed_usage_timestamp ON detailed_usage(timestamp);
CREATE INDEX IF NOT EXISTS idx_detailed_usage_exit_code ON detailed_usage(exit_code) WHERE exit_code IS NOT NULL AND exit_code != 0;

-- Hourly aggregations for better granularity
CREATE TABLE IF NOT EXISTS hourly_usage_metrics (
    id BIGSERIAL PRIMARY KEY,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    hour TIMESTAMPTZ NOT NULL,
    executions INTEGER NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    memory_peak_total_mb BIGINT NOT NULL DEFAULT 0,
    cpu_total_millis BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT hourly_usage_api_key_hour_unique UNIQUE(api_key_id, hour),
    CONSTRAINT hourly_usage_executions_check CHECK (executions >= 0),
    CONSTRAINT hourly_usage_duration_check CHECK (duration_ms >= 0),
    CONSTRAINT hourly_usage_errors_check CHECK (errors >= 0)
);

-- Indexes for hourly metrics
CREATE INDEX IF NOT EXISTS idx_hourly_usage_api_key_hour ON hourly_usage_metrics(api_key_id, hour DESC);
CREATE INDEX IF NOT EXISTS idx_hourly_usage_hour ON hourly_usage_metrics(hour);

-- Key usage audit log
CREATE TABLE IF NOT EXISTS api_key_audit_log (
    id BIGSERIAL PRIMARY KEY,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    action TEXT NOT NULL, -- created, updated, deleted, rotated, limits_changed
    details JSONB,
    performed_by TEXT NOT NULL, -- API key that performed the action
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT api_key_audit_log_action_check CHECK (action IN ('created', 'updated', 'deleted', 'rotated', 'limits_changed', 'activated', 'deactivated'))
);

-- Indexes for audit log
CREATE INDEX IF NOT EXISTS idx_api_key_audit_log_key_id ON api_key_audit_log(api_key_id);
CREATE INDEX IF NOT EXISTS idx_api_key_audit_log_timestamp ON api_key_audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_api_key_audit_log_action ON api_key_audit_log(action);

-- Comments
COMMENT ON COLUMN api_keys.name IS 'Human-readable name for the API key';
COMMENT ON COLUMN api_keys.description IS 'Optional description of the key purpose';
COMMENT ON COLUMN api_keys.is_active IS 'Whether the key is currently active';
COMMENT ON COLUMN api_keys.expires_at IS 'Optional expiration date for the key';
COMMENT ON COLUMN api_keys.parent_key_id IS 'For hierarchical key management (parent account key)';
COMMENT ON COLUMN api_keys.custom_daily_limit IS 'Customer-defined daily execution limit';
COMMENT ON COLUMN api_keys.custom_concurrent_limit IS 'Customer-defined concurrent session limit';
COMMENT ON COLUMN api_keys.last_updated_by IS 'API key that last updated this key';
COMMENT ON COLUMN api_keys.metadata IS 'Flexible JSON storage for custom key attributes';

COMMENT ON TABLE detailed_usage IS 'Detailed execution metrics for analytics and billing';
COMMENT ON TABLE hourly_usage_metrics IS 'Hourly aggregated usage metrics for performance queries';
COMMENT ON TABLE api_key_audit_log IS 'Audit trail for all API key management operations';

-- Function to update hourly metrics (called by triggers or scheduled job)
CREATE OR REPLACE FUNCTION update_hourly_usage_metrics()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO hourly_usage_metrics (
        api_key_id, 
        hour, 
        executions, 
        duration_ms, 
        errors, 
        memory_peak_total_mb, 
        cpu_total_millis,
        updated_at
    ) VALUES (
        NEW.api_key_id,
        date_trunc('hour', NEW.timestamp),
        1,
        NEW.duration_ms,
        CASE WHEN NEW.exit_code IS NOT NULL AND NEW.exit_code != 0 THEN 1 ELSE 0 END,
        COALESCE(NEW.memory_peak_mb, 0),
        COALESCE(NEW.cpu_millis, 0),
        NOW()
    )
    ON CONFLICT (api_key_id, hour) DO UPDATE SET
        executions = hourly_usage_metrics.executions + 1,
        duration_ms = hourly_usage_metrics.duration_ms + NEW.duration_ms,
        errors = hourly_usage_metrics.errors + CASE WHEN NEW.exit_code IS NOT NULL AND NEW.exit_code != 0 THEN 1 ELSE 0 END,
        memory_peak_total_mb = hourly_usage_metrics.memory_peak_total_mb + COALESCE(NEW.memory_peak_mb, 0),
        cpu_total_millis = hourly_usage_metrics.cpu_total_millis + COALESCE(NEW.cpu_millis, 0),
        updated_at = NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update hourly metrics
CREATE TRIGGER trigger_update_hourly_usage_metrics
    AFTER INSERT ON detailed_usage
    FOR EACH ROW
    EXECUTE FUNCTION update_hourly_usage_metrics();

-- Function to add audit log entry
CREATE OR REPLACE FUNCTION add_api_key_audit_log()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO api_key_audit_log (api_key_id, action, details, performed_by)
        VALUES (NEW.id, 'created', 
                jsonb_build_object('name', NEW.name, 'tier', NEW.tier), 
                NEW.last_updated_by);
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        -- Only log if key fields actually changed
        IF OLD.name IS DISTINCT FROM NEW.name OR 
           OLD.description IS DISTINCT FROM NEW.description OR
           OLD.is_active IS DISTINCT FROM NEW.is_active OR
           OLD.custom_daily_limit IS DISTINCT FROM NEW.custom_daily_limit OR
           OLD.custom_concurrent_limit IS DISTINCT FROM NEW.custom_concurrent_limit THEN
            INSERT INTO api_key_audit_log (api_key_id, action, details, performed_by)
            VALUES (NEW.id, 'updated', 
                    jsonb_build_object(
                        'old_name', OLD.name,
                        'new_name', NEW.name,
                        'old_description', OLD.description,
                        'new_description', NEW.description,
                        'old_is_active', OLD.is_active,
                        'new_is_active', NEW.is_active,
                        'old_daily_limit', OLD.custom_daily_limit,
                        'new_daily_limit', NEW.custom_daily_limit,
                        'old_concurrent_limit', OLD.custom_concurrent_limit,
                        'new_concurrent_limit', NEW.custom_concurrent_limit
                    ), 
                    NEW.last_updated_by);
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO api_key_audit_log (api_key_id, action, details, performed_by)
        VALUES (OLD.id, 'deleted', 
                jsonb_build_object('name', OLD.name, 'tier', OLD.tier), 
                OLD.last_updated_by);
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Trigger for audit logging
CREATE TRIGGER trigger_api_key_audit_log
    AFTER INSERT OR UPDATE OR DELETE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION add_api_key_audit_log();