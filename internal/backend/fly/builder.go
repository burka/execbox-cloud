package fly

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const (
	imagePrefix = "execbox-"
	hashLength  = 16 // 64 bits, collision at ~4 billion images
)

// BuildFile represents a file to include in the Docker build context.
type BuildFile struct {
	Path    string // Path in container
	Content []byte // File content
}

// BuildSpec contains all the information needed to build an image.
type BuildSpec struct {
	BaseImage string      // Base image (e.g., "python:3.12")
	Setup     []string    // RUN commands to execute
	Files     []BuildFile // Files to include in image
}

// ComputeHash generates a content-addressed hash from the build spec.
// Returns a 16-character hex string.
func ComputeHash(spec *BuildSpec) string {
	h := sha256.New()

	// Hash base image
	h.Write([]byte(spec.BaseImage))
	h.Write([]byte{0}) // Separator

	// Hash setup commands in order
	for _, cmd := range spec.Setup {
		h.Write([]byte(cmd))
		h.Write([]byte{0})
	}

	// Hash files (sorted by path for determinism)
	if len(spec.Files) > 0 {
		sortedFiles := make([]BuildFile, len(spec.Files))
		copy(sortedFiles, spec.Files)
		sort.Slice(sortedFiles, func(i, j int) bool {
			return sortedFiles[i].Path < sortedFiles[j].Path
		})

		for _, f := range sortedFiles {
			h.Write([]byte(f.Path))
			h.Write([]byte{0})
			h.Write(f.Content)
			h.Write([]byte{0})
		}
	}

	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:hashLength/2])
}

// ImageTag returns the full image tag for a given hash.
// Format: execbox-<hash>
func ImageTag(hash string) string {
	return imagePrefix + hash
}

// RegistryTag returns the full registry path for an image.
// Format: registry.fly.io/<app>/execbox-<hash>
func RegistryTag(appName, hash string) string {
	return fmt.Sprintf("registry.fly.io/%s/%s", appName, ImageTag(hash))
}

// GenerateDockerfile creates a Dockerfile from a build spec.
func GenerateDockerfile(spec *BuildSpec) string {
	var b strings.Builder

	// FROM base image
	b.WriteString(fmt.Sprintf("FROM %s\n", spec.BaseImage))

	// COPY files (if any)
	for _, f := range spec.Files {
		b.WriteString(fmt.Sprintf("COPY %s %s\n", f.Path, f.Path))
	}

	// RUN setup commands
	for _, cmd := range spec.Setup {
		b.WriteString(fmt.Sprintf("RUN %s\n", cmd))
	}

	return b.String()
}

// ValidateSetup checks setup commands for injection attacks.
// Returns error if setup contains FROM directive.
func ValidateSetup(setup []string) error {
	for _, cmd := range setup {
		upper := strings.ToUpper(strings.TrimSpace(cmd))
		if strings.HasPrefix(upper, "FROM ") || upper == "FROM" {
			return fmt.Errorf("FROM directive not allowed in setup")
		}
	}
	return nil
}

// Builder handles image building and caching for Fly.io.
type Builder struct {
	client  *Client
	appName string
}

// NewBuilder creates a new Builder instance.
func NewBuilder(client *Client, appName string) *Builder {
	return &Builder{
		client:  client,
		appName: appName,
	}
}

// Resolve resolves a build spec to a registry tag.
// Returns the base image unchanged if no setup/files are provided.
// Otherwise, builds the image (if not cached) and returns the registry tag.
func (b *Builder) Resolve(ctx context.Context, spec *BuildSpec, cache BuildCache) (string, error) {
	// Fast path: no setup and no files means use base image directly
	if len(spec.Setup) == 0 && len(spec.Files) == 0 {
		return spec.BaseImage, nil
	}

	// Validate base image
	if strings.TrimSpace(spec.BaseImage) == "" {
		return "", fmt.Errorf("base image cannot be empty when setup is provided")
	}

	// Validate setup commands
	if err := ValidateSetup(spec.Setup); err != nil {
		return "", err
	}

	// Compute content-addressed hash
	hash := ComputeHash(spec)

	// Check cache
	if cache != nil {
		if registryTag, ok, err := cache.Get(ctx, hash); err != nil {
			return "", fmt.Errorf("check cache: %w", err)
		} else if ok {
			// Update last_used_at in background
			go func() {
				_ = cache.Touch(context.Background(), hash)
			}()
			return registryTag, nil
		}
	}

	// Build the image
	registryTag := RegistryTag(b.appName, hash)
	if err := b.build(ctx, spec, registryTag); err != nil {
		return "", fmt.Errorf("build image: %w", err)
	}

	// Store in cache
	if cache != nil {
		if err := cache.Put(ctx, hash, spec.BaseImage, registryTag); err != nil {
			// Log but don't fail - image is built, just not cached
			// TODO: Add proper logging
		}
	}

	return registryTag, nil
}

// build builds and pushes an image to the Fly registry.
// This uses the Fly Machines API to create a temporary builder machine.
func (b *Builder) build(ctx context.Context, spec *BuildSpec, registryTag string) error {
	// For now, we'll use the Fly CLI approach via subprocess
	// In production, this would use the Fly API directly
	//
	// TODO: Implement proper Fly remote builder integration
	// Options:
	// 1. fly deploy --build-only (requires Dockerfile in temp dir)
	// 2. Fly Machines API with builder image
	// 3. Dedicated BuildKit machine

	return fmt.Errorf("image building not yet implemented - use pre-built images for now")
}

// BuildCache is the interface for image cache storage.
type BuildCache interface {
	// Get returns the registry tag for a hash if it exists.
	Get(ctx context.Context, hash string) (registryTag string, ok bool, err error)

	// Put stores a new cache entry.
	Put(ctx context.Context, hash, baseImage, registryTag string) error

	// Touch updates the last_used_at timestamp for a hash.
	Touch(ctx context.Context, hash string) error
}
