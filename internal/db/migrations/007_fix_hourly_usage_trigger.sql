-- Migration: Fix hourly usage trigger to prevent double-counting
-- Only count session once when it transitions to terminal state

-- Drop the old trigger
DROP TRIGGER IF EXISTS trigger_update_hourly_account_usage ON sessions;

-- Create improved trigger that only fires on terminal state transition
CREATE OR REPLACE FUNCTION update_hourly_account_usage()
RETURNS TRIGGER AS $$
BEGIN
    -- Only insert/update metrics when session ends (transitions to terminal state)
    IF TG_OP = 'UPDATE' AND 
       OLD.status NOT IN ('stopped', 'failed', 'killed') AND 
       NEW.status IN ('stopped', 'failed', 'killed') THEN
        
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
            duration_ms = hourly_account_usage.duration_ms + EXCLUDED.duration_ms,
            cost_estimate_cents = hourly_account_usage.cost_estimate_cents + EXCLUDED.cost_estimate_cents,
            cpu_millis_used = hourly_account_usage.cpu_millis_used + EXCLUDED.cpu_millis_used,
            memory_mb_seconds = hourly_account_usage.memory_mb_seconds + EXCLUDED.memory_mb_seconds,
            errors = hourly_account_usage.errors + EXCLUDED.errors,
            updated_at = NOW();
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate trigger - only fires on UPDATE now
CREATE TRIGGER trigger_update_hourly_account_usage
    AFTER UPDATE ON sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_hourly_account_usage();