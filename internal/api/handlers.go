package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/fly"
	"github.com/burka/execbox-cloud/internal/db"
	"github.com/go-chi/chi/v5"
)

// FlyClient defines the Fly.io operations required by handlers.
type FlyClient interface {
	CreateMachine(ctx context.Context, config *fly.MachineConfig) (*fly.Machine, error)
	StopMachine(ctx context.Context, machineID string) error
	DestroyMachine(ctx context.Context, machineID string) error
}

// ImageBuilder defines the image building operations.
type ImageBuilder interface {
	Resolve(ctx context.Context, spec *fly.BuildSpec, cache fly.BuildCache) (string, error)
}

// Handlers holds the HTTP request handlers and their dependencies.
type Handlers struct {
	db      DBClient
	fly     FlyClient
	builder ImageBuilder
	cache   fly.BuildCache
}

// NewHandlers creates a new Handlers instance with the provided database and Fly clients.
func NewHandlers(dbClient DBClient, flyClient FlyClient) *Handlers {
	return &Handlers{
		db:  dbClient,
		fly: flyClient,
	}
}

// SetBuilder configures the image builder for handlers that need it.
func (h *Handlers) SetBuilder(builder ImageBuilder, cache fly.BuildCache) {
	h.builder = builder
	h.cache = cache
}

// getAuthorizedSession retrieves a session and verifies the caller owns it.
// Returns the session or writes an error response and returns nil.
func (h *Handlers) getAuthorizedSession(w http.ResponseWriter, r *http.Request) *db.Session {
	ctx := r.Context()

	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return nil
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		WriteError(w, fmt.Errorf("%w: session ID is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return nil
	}

	session, err := h.db.GetSession(ctx, sessionID)
	if err != nil {
		WriteError(w, ErrNotFound, http.StatusNotFound, CodeNotFound)
		return nil
	}

	if session.APIKeyID != apiKeyID {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return nil
	}

	return session
}

// CreateSession handles POST /v1/sessions
// Creates a new execution session with a Fly machine and stores it in the database.
func (h *Handlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID from context (set by auth middleware)
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
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
		activeCount, err := h.db.GetActiveSessionCount(ctx, apiKeyID)
		if err != nil {
			WriteError(w, fmt.Errorf("%w: failed to check concurrent sessions: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
			return
		}

		if activeCount >= limits.ConcurrentSessions {
			WriteError(w, fmt.Errorf("%w: concurrent session limit reached (%d/%d)", ErrQuotaExceeded, activeCount, limits.ConcurrentSessions), http.StatusTooManyRequests, CodeQuotaExceeded)
			return
		}
	}

	// Check daily session limit
	if !IsUnlimited(limits.SessionsPerDay) {
		dailyCount, err := h.db.GetDailySessionCount(ctx, apiKeyID)
		if err != nil {
			WriteError(w, fmt.Errorf("%w: failed to check daily sessions: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
			return
		}

		if dailyCount >= limits.SessionsPerDay {
			WriteError(w, fmt.Errorf("%w: daily session limit reached (%d/%d)", ErrQuotaExceeded, dailyCount, limits.SessionsPerDay), http.StatusTooManyRequests, CodeQuotaExceeded)
			return
		}
	}

	// Parse request body
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, fmt.Errorf("%w: invalid JSON", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Validate required fields
	if req.Image == "" {
		WriteError(w, fmt.Errorf("%w: image is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Reject setup/files until image building is fully implemented
	if len(req.Setup) > 0 || len(req.Files) > 0 {
		WriteError(w, fmt.Errorf("%w: custom image building (setup/files) not yet available", ErrBadRequest),
			http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Generate session ID
	sessionID := generateSessionID()

	// Resolve image (build if setup/files provided)
	resolvedImage := req.Image
	var setupHash string
	if len(req.Setup) > 0 || len(req.Files) > 0 {
		if h.builder == nil {
			WriteError(w, fmt.Errorf("%w: image building not configured", ErrInternal), http.StatusInternalServerError, CodeInternal)
			return
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
		resolvedImage, err = h.builder.Resolve(ctx, spec, h.cache)
		if err != nil {
			WriteError(w, fmt.Errorf("%w: failed to resolve image: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
			return
		}
	}

	// Build Fly machine configuration with resolved image
	machineConfig := buildMachineConfig(&req)
	machineConfig.Image = resolvedImage

	// Create Fly machine
	machine, err := h.fly.CreateMachine(ctx, machineConfig)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to create machine: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build ports from request
	ports := buildPorts(req.Ports)

	// Create session in database
	session := &db.Session{
		ID:           sessionID,
		APIKeyID:     apiKeyID,
		FlyMachineID: &machine.ID,
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

	if err := h.db.CreateSession(ctx, session); err != nil {
		WriteError(w, fmt.Errorf("%w: failed to create session: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := CreateSessionResponse{
		ID:        sessionID,
		Status:    SessionStatusPending,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
	}

	// Add network info if ports are specified
	if len(req.Ports) > 0 && req.Network != "" && req.Network != "none" {
		response.Network = buildNetworkInfo(req.Network, ports, machine)
	}

	_ = WriteJSON(w, response, http.StatusCreated)
}

// GetSession handles GET /v1/sessions/{id}
// Retrieves a session by ID, checking ownership.
func (h *Handlers) GetSession(w http.ResponseWriter, r *http.Request) {
	session := h.getAuthorizedSession(w, r)
	if session == nil {
		return // Error already written
	}

	response := buildSessionResponse(session)
	_ = WriteJSON(w, response, http.StatusOK)
}

// ListSessions handles GET /v1/sessions
// Lists all sessions for the authenticated API key, with optional status filter.
func (h *Handlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Get optional status filter from query params
	var statusFilter *string
	if status := r.URL.Query().Get("status"); status != "" {
		statusFilter = &status
	}

	// List sessions from database
	sessions, err := h.db.ListSessions(ctx, apiKeyID, statusFilter)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to list sessions: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := ListSessionsResponse{
		Sessions: make([]SessionResponse, 0, len(sessions)),
	}

	for _, session := range sessions {
		response.Sessions = append(response.Sessions, buildSessionResponse(&session))
	}

	_ = WriteJSON(w, response, http.StatusOK)
}

// StopSession handles POST /v1/sessions/{id}/stop
// Stops a running session by stopping the Fly machine and updating the database.
func (h *Handlers) StopSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := h.getAuthorizedSession(w, r)
	if session == nil {
		return // Error already written
	}

	// Check if already stopped
	if session.Status == SessionStatusStopped || session.Status == SessionStatusKilled || session.Status == SessionStatusFailed {
		WriteError(w, ErrConflict, http.StatusConflict, CodeConflict)
		return
	}

	// Stop Fly machine if it exists
	if session.FlyMachineID != nil {
		if err := h.fly.StopMachine(ctx, *session.FlyMachineID); err != nil {
			WriteError(w, fmt.Errorf("%w: failed to stop machine: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
			return
		}
	}

	// Update session status in database
	now := time.Now().UTC()
	status := SessionStatusStopped
	update := &db.SessionUpdate{
		Status:  &status,
		EndedAt: &now,
	}

	if err := h.db.UpdateSession(ctx, session.ID, update); err != nil {
		WriteError(w, fmt.Errorf("%w: failed to update session: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := StopSessionResponse{
		Status: SessionStatusStopped,
	}

	_ = WriteJSON(w, response, http.StatusOK)
}

// KillSession handles DELETE /v1/sessions/{id}
// Permanently deletes a session by destroying the Fly machine and updating the database.
func (h *Handlers) KillSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := h.getAuthorizedSession(w, r)
	if session == nil {
		return // Error already written
	}

	// Destroy Fly machine if it exists
	if session.FlyMachineID != nil {
		if err := h.fly.DestroyMachine(ctx, *session.FlyMachineID); err != nil {
			WriteError(w, fmt.Errorf("%w: failed to destroy machine: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
			return
		}
	}

	// Update session status in database
	now := time.Now().UTC()
	status := SessionStatusKilled
	update := &db.SessionUpdate{
		Status:  &status,
		EndedAt: &now,
	}

	if err := h.db.UpdateSession(ctx, session.ID, update); err != nil {
		WriteError(w, fmt.Errorf("%w: failed to update session: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := StopSessionResponse{
		Status: SessionStatusKilled,
	}

	_ = WriteJSON(w, response, http.StatusOK)
}

// generateSessionID creates a random session ID with the format "sess_<random>"
func generateSessionID() string {
	return fmt.Sprintf("sess_%s", randHex(12))
}

// randHex generates a random hex string of the specified length
func randHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand failure indicates system compromise or severe misconfiguration
		// We must never fall back to predictable IDs - panic is the only safe response
		panic(fmt.Sprintf("crypto/rand failed: %v - system security compromised", err))
	}
	return hex.EncodeToString(bytes)[:n]
}

// buildMachineConfig creates a Fly machine configuration from a CreateSessionRequest
func buildMachineConfig(req *CreateSessionRequest) *fly.MachineConfig {
	config := &fly.MachineConfig{
		Image:       req.Image,
		Cmd:         req.Command,
		Env:         req.Env,
		AutoDestroy: false,
	}

	// Add resource configuration if specified
	if req.Resources != nil {
		config.Guest = &fly.Guest{}
		if req.Resources.CPUMillis > 0 {
			// Convert millicores to CPU count (Fly uses CPU count)
			config.Guest.CPUs = (req.Resources.CPUMillis + 999) / 1000
		}
		if req.Resources.MemoryMB > 0 {
			config.Guest.MemoryMB = req.Resources.MemoryMB
		}
	}

	// Add service configuration for exposed ports
	if len(req.Ports) > 0 && req.Network == "exposed" {
		services := make([]fly.Service, 0, len(req.Ports))
		for _, port := range req.Ports {
			protocol := port.Protocol
			if protocol == "" {
				protocol = "tcp"
			}

			services = append(services, fly.Service{
				InternalPort: port.Container,
				Protocol:     protocol,
				Ports: []fly.ServicePort{
					{
						Port: port.Container,
					},
				},
			})
		}
		config.Services = services
	}

	return config
}

// buildPorts converts API PortSpec to database Port models
func buildPorts(specs []PortSpec) []db.Port {
	if len(specs) == 0 {
		return nil
	}

	ports := make([]db.Port, 0, len(specs))
	for _, spec := range specs {
		protocol := spec.Protocol
		if protocol == "" {
			protocol = "tcp"
		}

		ports = append(ports, db.Port{
			Container: spec.Container,
			Protocol:  protocol,
		})
	}

	return ports
}

// buildNetworkInfo creates network information from session data
func buildNetworkInfo(mode string, ports []db.Port, machine *fly.Machine) *NetworkInfo {
	if mode == "none" || len(ports) == 0 {
		return nil
	}

	info := &NetworkInfo{
		Mode:  mode,
		Ports: make(map[string]PortInfo),
	}

	// For exposed mode, populate host and port mappings
	if mode == "exposed" && machine != nil {
		// Use machine region as hostname (simplified - in production would use actual DNS)
		info.Host = fmt.Sprintf("%s.fly.dev", machine.Region)

		for _, port := range ports {
			portKey := fmt.Sprintf("%d", port.Container)
			info.Ports[portKey] = PortInfo{
				HostPort: port.Container,
				URL:      fmt.Sprintf("%s://%s:%d", port.Protocol, info.Host, port.Container),
			}
		}
	}

	return info
}

// CreateQuotaRequest handles POST /v1/quota-requests
// Creates a new quota request for users wanting to upgrade their tier.
// This endpoint is public (no auth required) but can optionally be used with auth.
func (h *Handlers) CreateQuotaRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req QuotaRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, fmt.Errorf("%w: invalid JSON", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" {
		WriteError(w, fmt.Errorf("%w: email is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Build quota request model
	quotaReq := &db.QuotaRequest{
		Email:           req.Email,
		Name:            req.Name,
		Company:         req.Company,
		UseCase:         req.UseCase,
		RequestedLimits: req.RequestedLimits,
		Budget:          req.Budget,
	}

	// If authenticated, attach API key info
	if apiKeyID, ok := GetAPIKeyID(ctx); ok {
		quotaReq.APIKeyID = &apiKeyID

		// Try to get current tier from context
		if tier, ok := GetAPIKeyTier(ctx); ok {
			quotaReq.CurrentTier = &tier
		}
	}

	// Create in database
	created, err := h.db.CreateQuotaRequest(ctx, quotaReq)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to create quota request: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := QuotaRequestResponse{
		ID:        created.ID,
		Status:    QuotaStatusPending,
		Message:   "Your quota request has been submitted. We'll review it and get back to you soon.",
		CreatedAt: created.CreatedAt.Format(time.RFC3339),
	}

	_ = WriteJSON(w, response, http.StatusCreated)
}

// GetAccount handles GET /v1/account
// Returns account information for the authenticated API key.
func (h *Handlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Get full API key details from database
	apiKey, err := h.db.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to get API key: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := AccountResponse{
		Tier:          apiKey.Tier,
		Email:         apiKey.Email,
		APIKeyID:      apiKey.ID.String(),
		APIKeyPreview: maskAPIKey(apiKey.Key),
		CreatedAt:     apiKey.CreatedAt.Format(time.RFC3339),
	}

	if apiKey.TierExpiresAt != nil {
		expiresAt := apiKey.TierExpiresAt.Format(time.RFC3339)
		response.TierExpiresAt = &expiresAt
	}

	_ = WriteJSON(w, response, http.StatusOK)
}

// GetUsage handles GET /v1/account/usage
// Returns usage statistics for the authenticated API key.
func (h *Handlers) GetUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID and tier from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	tier, ok := GetAPIKeyTier(ctx)
	if !ok {
		tier = TierFree // Default to free tier if not set
	}

	// Get session counts
	dailyCount, err := h.db.GetDailySessionCount(ctx, apiKeyID)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to get daily session count: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	activeCount, err := h.db.GetActiveSessionCount(ctx, apiKeyID)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to get active session count: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Get tier limits
	limits := GetTierLimits(tier)

	// Calculate quota remaining
	quotaRemaining := limits.SessionsPerDay - dailyCount
	if IsUnlimited(limits.SessionsPerDay) {
		quotaRemaining = -1 // Indicate unlimited
	} else if quotaRemaining < 0 {
		quotaRemaining = 0
	}

	// Build response
	response := UsageResponse{
		SessionsToday:      dailyCount,
		ActiveSessions:     activeCount,
		QuotaUsed:          dailyCount,
		QuotaRemaining:     quotaRemaining,
		Tier:               tier,
		ConcurrentLimit:    limits.ConcurrentSessions,
		DailyLimit:         limits.SessionsPerDay,
		MaxDurationSeconds: limits.MaxDurationSec,
		MaxMemoryMB:        limits.MemoryMB,
	}

	_ = WriteJSON(w, response, http.StatusOK)
}

// CreateAPIKey handles POST /v1/keys
// Creates a new API key (public endpoint, no auth required).
func (h *Handlers) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, fmt.Errorf("%w: invalid JSON", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" {
		WriteError(w, fmt.Errorf("%w: email is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Create API key in database
	apiKey, err := h.db.CreateAPIKey(ctx, req.Email, req.Name)
	if err != nil {
		WriteError(w, fmt.Errorf("%w: failed to create API key: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := CreateKeyResponse{
		ID:      apiKey.ID.String(),
		Key:     apiKey.Key,
		Tier:    apiKey.Tier,
		Message: "API key created successfully. Save this key - it will only be shown once.",
	}

	_ = WriteJSON(w, response, http.StatusCreated)
}

// maskAPIKey masks an API key to show only the first 7 and last 4 characters.
func maskAPIKey(key string) string {
	if len(key) < 12 {
		return "sk_****"
	}
	return key[:7] + "..." + key[len(key)-4:]
}

// buildSessionResponse creates a SessionResponse from a database Session
func buildSessionResponse(session *db.Session) SessionResponse {
	response := SessionResponse{
		ID:        session.ID,
		Status:    session.Status,
		Image:     session.Image,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
		ExitCode:  session.ExitCode,
	}

	if session.StartedAt != nil {
		startedAt := session.StartedAt.Format(time.RFC3339)
		response.StartedAt = &startedAt
	}

	if session.EndedAt != nil {
		endedAt := session.EndedAt.Format(time.RFC3339)
		response.EndedAt = &endedAt
	}

	// Build network info from ports if available
	if len(session.Ports) > 0 {
		portMap := make(map[string]PortInfo)
		for _, port := range session.Ports {
			portKey := fmt.Sprintf("%d", port.Container)
			info := PortInfo{
				HostPort: port.Host,
			}
			if port.URL != "" {
				info.URL = port.URL
			}
			portMap[portKey] = info
		}

		response.Network = &NetworkInfo{
			Mode:  "exposed", // Simplified - would track this in session
			Ports: portMap,
		}
	}

	return response
}
