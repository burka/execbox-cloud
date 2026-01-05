package fly

import "context"

// DBBuildCache implements BuildCache using a database client.
type DBBuildCache struct {
	get   func(ctx context.Context, hash string) (string, bool, error)
	put   func(ctx context.Context, hash, baseImage, registryTag string) error
	touch func(ctx context.Context, hash string) error
}

// NewDBBuildCache creates a new DBBuildCache from database methods.
// This adapter pattern allows the fly package to use db.Client without importing it.
func NewDBBuildCache(
	get func(ctx context.Context, hash string) (string, bool, error),
	put func(ctx context.Context, hash, baseImage, registryTag string) error,
	touch func(ctx context.Context, hash string) error,
) *DBBuildCache {
	return &DBBuildCache{
		get:   get,
		put:   put,
		touch: touch,
	}
}

// Get returns the registry tag for a hash if it exists.
func (c *DBBuildCache) Get(ctx context.Context, hash string) (string, bool, error) {
	return c.get(ctx, hash)
}

// Put stores a new cache entry.
func (c *DBBuildCache) Put(ctx context.Context, hash, baseImage, registryTag string) error {
	return c.put(ctx, hash, baseImage, registryTag)
}

// Touch updates the last_used_at timestamp for a hash.
func (c *DBBuildCache) Touch(ctx context.Context, hash string) error {
	return c.touch(ctx, hash)
}
