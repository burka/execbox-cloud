-- Migration: 002_image_cache
-- Description: Add image cache table for storing built images

-- ============================================================================
-- Image Cache Table
-- ============================================================================
-- Stores content-addressed images built from setup commands
-- Used to avoid rebuilding identical images

CREATE TABLE image_cache (
    id SERIAL PRIMARY KEY,
    hash TEXT UNIQUE NOT NULL,              -- Content-addressed hash (SHA256, 16 hex chars)
    base_image TEXT NOT NULL,               -- Original base image (e.g., "python:3.12")
    registry_tag TEXT NOT NULL,             -- Full registry tag (e.g., "registry.fly.io/execbox/execbox-abc123")
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,               -- Updated on cache hit
    build_duration_ms INTEGER,              -- Time taken to build (for metrics)

    CONSTRAINT image_cache_hash_format CHECK (length(hash) = 16)
);

-- Index for fast hash lookups
CREATE INDEX idx_image_cache_hash ON image_cache(hash);

-- Index for cleanup of stale images
CREATE INDEX idx_image_cache_last_used ON image_cache(last_used_at);

COMMENT ON TABLE image_cache IS 'Cache of built images to avoid rebuilding identical setups';
COMMENT ON COLUMN image_cache.hash IS 'Content-addressed SHA256 hash (16 hex chars)';
COMMENT ON COLUMN image_cache.base_image IS 'Original base image before setup';
COMMENT ON COLUMN image_cache.registry_tag IS 'Full registry tag for the built image';
COMMENT ON COLUMN image_cache.last_used_at IS 'Timestamp of last cache hit (for cleanup)';
COMMENT ON COLUMN image_cache.build_duration_ms IS 'Build time in milliseconds (for metrics)';
