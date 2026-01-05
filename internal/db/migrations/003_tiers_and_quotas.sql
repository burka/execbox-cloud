-- Migration: 003_tiers_and_quotas
-- Description: Add tier system for quota management (no billing yet)

-- Add tier and contact fields to api_keys
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS tier TEXT NOT NULL DEFAULT 'free';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS tier_expires_at TIMESTAMPTZ;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS tier_updated_at TIMESTAMPTZ;

-- Index for tier queries
CREATE INDEX IF NOT EXISTS idx_api_keys_tier ON api_keys(tier);

-- Index for email lookups (for login)
CREATE INDEX IF NOT EXISTS idx_api_keys_email ON api_keys(email) WHERE email IS NOT NULL;

-- Constraint for valid tiers (free for now, expand later)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'api_keys_tier_valid'
    ) THEN
        ALTER TABLE api_keys ADD CONSTRAINT api_keys_tier_valid
        CHECK (tier IN ('free', 'starter', 'pro', 'enterprise'));
    END IF;
END $$;

COMMENT ON COLUMN api_keys.email IS 'User email for notifications and login';
COMMENT ON COLUMN api_keys.tier IS 'Subscription tier: free, starter, pro, enterprise';
COMMENT ON COLUMN api_keys.tier_expires_at IS 'When current tier expires (for manual upgrades)';
COMMENT ON COLUMN api_keys.tier_updated_at IS 'When tier was last changed';

-- Quota request table for upgrade requests
CREATE TABLE IF NOT EXISTS quota_requests (
    id SERIAL PRIMARY KEY,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    email TEXT NOT NULL,
    name TEXT,
    company TEXT,
    current_tier TEXT,
    requested_limits TEXT,  -- Free text: what they need
    budget TEXT,            -- Free text: what they're willing to pay
    use_case TEXT,          -- Free text: what they're building
    status TEXT NOT NULL DEFAULT 'pending',
    notes TEXT,             -- Internal notes
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ,

    CONSTRAINT quota_requests_status_valid CHECK (status IN ('pending', 'contacted', 'converted', 'declined'))
);

CREATE INDEX IF NOT EXISTS idx_quota_requests_status ON quota_requests(status);
CREATE INDEX IF NOT EXISTS idx_quota_requests_email ON quota_requests(email);

COMMENT ON TABLE quota_requests IS 'Upgrade requests from users wanting more quota';
