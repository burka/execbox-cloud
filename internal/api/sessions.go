package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/danielgtaylor/huma/v2"
)

// SessionService handles session-related operations.
// It implements huma handler functions for session management.
type SessionService struct {
	db      DBClient
	backend Backend
	builder ImageBuilder
	cache   fly.BuildCache
}

// NewSessionService creates a new SessionService.
func NewSessionService(db DBClient, backend Backend) *SessionService {
	return &SessionService{
		db:      db,
		backend: backend,
	}
}

// SetBuilder sets the image builder and cache for custom images.
func (s *SessionService) SetBuilder(builder ImageBuilder, cache fly.BuildCache) {
	s.builder = builder
	s.cache = cache
}

// getAuthorizedSession retrieves a session and verifies the caller owns it.
// Returns the session or an error if not found or unauthorized.
func (s *SessionService) getAuthorizedSession(ctx context.Context, sessionID string) (*db.Session, error) {
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	if sessionID == "" {
		return nil, huma.Error400BadRequest("session ID is required")
	}

	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		return nil, huma.Error404NotFound("session not found")
	}

	if session.APIKeyID != apiKeyID {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	return session, nil
}

// CreateSession handles POST /v1/sessions
// Creates a new execution session with a backend and stores it in the database.
func (s *SessionService) CreateSession(ctx context.Context, input *CreateSessionInput) (*CreateSessionOutput, error) {
	// Get API key ID from context (set by auth middleware)
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get tier from context
	tier, ok := GetAPIKeyTier(ctx)
	if !ok {
		// Default to anonymous tier if not authenticated
		tier = TierAnonymous
	}

	// Check quota limits before creating session
	limits := GetTierLimits(tier)

	// Check concurrent session limit
	if !IsUnlimited(limits.ConcurrentSessions) {
		activeCount, err := s.db.GetActiveSessionCount(ctx, apiKeyID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to check concurrent sessions: %v", err))
		}

		if activeCount >= limits.ConcurrentSessions {
			return nil, huma.NewError(http.StatusTooManyRequests, fmt.Sprintf("concurrent session limit reached (%d/%d)", activeCount, limits.ConcurrentSessions))
		}
	}

	// Check daily session limit
	if !IsUnlimited(limits.SessionsPerDay) {
		dailyCount, err := s.db.GetDailySessionCount(ctx, apiKeyID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to check daily sessions: %v", err))
		}

		if dailyCount >= limits.SessionsPerDay {
			return nil, huma.NewError(http.StatusTooManyRequests, fmt.Sprintf("daily session limit reached (%d/%d)", dailyCount, limits.SessionsPerDay))
		}
	}

	req := input.Body

	// Validate required fields
	if req.Image == "" {
		return nil, huma.Error400BadRequest("image is required")
	}

	// Reject setup/files until image building is fully implemented
	if len(req.Setup) > 0 || len(req.Files) > 0 {
		return nil, huma.Error400BadRequest("custom image building (setup/files) not yet available")
	}

	// Generate session ID
	sessionID := generateSessionID()

	// Resolve image (build if setup/files provided)
	resolvedImage := req.Image
	var setupHash string
	if len(req.Setup) > 0 || len(req.Files) > 0 {
		if s.builder == nil {
			return nil, huma.Error500InternalServerError("image building not configured")
		}

		// Build the spec
		spec := &fly.BuildSpec{
			BaseImage: req.Image,
			Setup:     req.Setup,
			Files:     make([]fly.BuildFile, 0, len(req.Files)),
		}

		for _, f := range req.Files {
			content := []byte(f.Content)
			// TODO: Handle base64 encoding if f.Encoding == "base64"
			spec.Files = append(spec.Files, fly.BuildFile{
				Path:    f.Path,
				Content: content,
			})
		}

		// Compute hash for tracking
		setupHash = fly.ComputeHash(spec)

		// Resolve to registry tag
		var err error
		resolvedImage, err = s.builder.Resolve(ctx, spec, s.cache)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to resolve image: %v", err))
		}
	}

	// Build ports from request
	ports := buildPorts(req.Ports)

	// Create session using backend interface
	if s.backend == nil {
		return nil, huma.Error500InternalServerError("no backend configured")
	}

	config := buildCreateSessionConfig(&req, resolvedImage)
	backendSession, backendNetwork, err := s.backend.CreateSession(ctx, config)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to create session: %v", err))
	}
	backendID := backendSession.BackendID

	// Convert backend network info to API response format
	var networkInfo *NetworkInfo
	if backendNetwork != nil && len(req.Ports) > 0 && req.Network != "" && req.Network != "none" {
		networkInfo = &NetworkInfo{
			Mode:  backendNetwork.Mode,
			Host:  backendNetwork.Host,
			Ports: make(map[string]PortInfo),
		}
		for containerPort, portInfo := range backendNetwork.Ports {
			portKey := fmt.Sprintf("%d", containerPort)
			networkInfo.Ports[portKey] = PortInfo{
				HostPort: portInfo.HostPort,
				URL:      portInfo.URL,
			}
		}
	}

	// Create session in database
	// Note: We use FlyMachineID for backward compatibility until backend_id column migration
	session := &db.Session{
		ID:           sessionID,
		APIKeyID:     apiKeyID,
		BackendID:    &backendID,
		FlyMachineID: &backendID, // Also set for DB compatibility
		Image:        resolvedImage,
		Command:      req.Command,
		Env:          req.Env,
		Status:       SessionStatusPending,
		Ports:        ports,
		CreatedAt:    time.Now().UTC(),
	}
	if setupHash != "" {
		session.SetupHash = &setupHash
	}

	if err := s.db.CreateSession(ctx, session); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to create session: %v", err))
	}

	// Build response
	response := CreateSessionResponse{
		ID:        sessionID,
		Status:    SessionStatusPending,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
	}

	// Add network info if available
	if networkInfo != nil {
		response.Network = networkInfo
	}

	return &CreateSessionOutput{Body: response}, nil
}

// GetSession handles GET /v1/sessions/{id}
// Retrieves a session by ID, checking ownership.
// Syncs status from backend if session is active and has a backend ID.
func (s *SessionService) GetSession(ctx context.Context, input *GetSessionInput) (*GetSessionOutput, error) {
	session, err := s.getAuthorizedSession(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	// Sync status from backend if session is active and has a backend ID
	backendID := session.GetBackendID()
	if backendID != "" && isActiveStatus(session.Status) {
		s.syncSessionStatus(ctx, session)
	}

	response := buildSessionResponse(session)
	return &GetSessionOutput{Body: response}, nil
}

// ListSessions handles GET /v1/sessions
// Lists all sessions for the authenticated API key, with optional status filter.
func (s *SessionService) ListSessions(ctx context.Context, input *ListSessionsInput) (*ListSessionsOutput, error) {
	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	// Get optional status filter from query params
	// TODO: Add status query parameter to ListSessionsInput when needed
	var statusFilter *string

	// List sessions from database
	sessions, err := s.db.ListSessions(ctx, apiKeyID, statusFilter)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to list sessions: %v", err))
	}

	// Build response
	response := ListSessionsResponse{
		Sessions: make([]SessionResponse, 0, len(sessions)),
	}

	for _, session := range sessions {
		response.Sessions = append(response.Sessions, buildSessionResponse(&session))
	}

	return &ListSessionsOutput{Body: response}, nil
}

// StopSession handles POST /v1/sessions/{id}/stop
// Stops a running session by stopping the backend session and updating the database.
func (s *SessionService) StopSession(ctx context.Context, input *StopSessionInput) (*StopSessionOutput, error) {
	session, err := s.getAuthorizedSession(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	// Check if already stopped
	if session.Status == SessionStatusStopped || session.Status == SessionStatusKilled || session.Status == SessionStatusFailed {
		return nil, huma.Error409Conflict("session already stopped")
	}

	// Stop the backend session
	backendID := session.GetBackendID()
	if backendID != "" && s.backend != nil {
		if err := s.backend.StopSession(ctx, backendID); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to stop session: %v", err))
		}
	}

	// Update session status in database
	now := time.Now().UTC()
	status := SessionStatusStopped
	update := &db.SessionUpdate{
		Status:  &status,
		EndedAt: &now,
	}

	if err := s.db.UpdateSession(ctx, session.ID, update); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to update session: %v", err))
	}

	// Build response
	response := StopSessionResponse{
		Status: SessionStatusStopped,
	}

	return &StopSessionOutput{Body: response}, nil
}

// KillSession handles DELETE /v1/sessions/{id}
// Permanently deletes a session by destroying the backend session and updating the database.
func (s *SessionService) KillSession(ctx context.Context, input *KillSessionInput) (*KillSessionOutput, error) {
	session, err := s.getAuthorizedSession(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	// Destroy the backend session
	backendID := session.GetBackendID()
	if backendID != "" && s.backend != nil {
		if err := s.backend.DestroySession(ctx, backendID); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to destroy session: %v", err))
		}
	}

	// Update session status in database
	now := time.Now().UTC()
	status := SessionStatusKilled
	update := &db.SessionUpdate{
		Status:  &status,
		EndedAt: &now,
	}

	if err := s.db.UpdateSession(ctx, session.ID, update); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to update session: %v", err))
	}

	return &KillSessionOutput{}, nil
}

// isActiveStatus checks if a status requires backend synchronization.
// Active statuses are those where the session may still be transitioning.
func isActiveStatus(status string) bool {
	return status == SessionStatusPending || status == SessionStatusRunning
}

// syncSessionStatus synchronizes session status from the backend.
// Updates the database if the backend reports a different status.
// This is called during GetSession to ensure status is current.
func (s *SessionService) syncSessionStatus(ctx context.Context, session *db.Session) {
	if s.backend == nil {
		return
	}

	backendID := session.GetBackendID()
	if backendID == "" {
		return
	}

	backendSession, err := s.backend.GetSession(ctx, backendID)
	if err != nil {
		// Log but don't fail - DB status is still valid
		return
	}

	// Update DB if status changed
	if backendSession.Status != session.Status {
		update := &db.SessionUpdate{Status: &backendSession.Status}

		// Copy exit code if available
		if backendSession.ExitCode != nil {
			update.ExitCode = backendSession.ExitCode
		}

		// Update timestamps if transitioning to terminal state
		if backendSession.Status == SessionStatusStopped || backendSession.Status == SessionStatusFailed {
			now := time.Now().UTC()
			update.EndedAt = &now

			// Calculate duration from CreatedAt to now
			durationMs := now.Sub(session.CreatedAt).Milliseconds()
			update.DurationMs = &durationMs

			// Calculate cost using default values since we don't have real metrics yet
			// cpuMillis = durationMs (assumes 1 core)
			// memoryMB = 256 (default container memory)
			cpuMillis := durationMs
			memoryMB := int64(256)

			update.CPUMillisUsed = &cpuMillis
			update.MemoryPeakMB = &memoryMB

			// Calculate cost using DefaultCostCalculator
			costEstimateCents := DefaultCostCalculator.CalculateSessionCost(durationMs, cpuMillis, memoryMB)
			update.CostEstimateCents = &costEstimateCents
		}

		// Update the database
		if err := s.db.UpdateSession(ctx, session.ID, update); err == nil {
			// Update local session object for response
			session.Status = backendSession.Status
			session.ExitCode = backendSession.ExitCode
			if update.EndedAt != nil {
				session.EndedAt = update.EndedAt
			}
		}
	}
}
