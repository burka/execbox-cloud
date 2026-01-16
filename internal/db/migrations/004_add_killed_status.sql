-- Migration 004: Add 'killed' status to sessions table
-- This status indicates a forceful termination (DELETE endpoint) vs graceful stop

-- Drop and recreate the constraint with the additional status
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_status_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_status_check
    CHECK (status IN ('pending', 'running', 'stopped', 'killed', 'failed'));
