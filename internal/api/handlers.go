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

// Handlers holds the HTTP request handlers and their dependencies.
type Handlers struct {
	db  DBClient
	fly FlyClient
}

// NewHandlers creates a new Handlers instance with the provided database and Fly clients.
func NewHandlers(dbClient DBClient, flyClient FlyClient) *Handlers {
	return &Handlers{
		db:  dbClient,
		fly: flyClient,
	}
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

	// Generate session ID
	sessionID := generateSessionID()

	// Build Fly machine configuration
	machineConfig := buildMachineConfig(&req)

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
		Image:        req.Image,
		Command:      req.Command,
		Env:          req.Env,
		Status:       "pending",
		Ports:        ports,
		CreatedAt:    time.Now().UTC(),
	}

	if err := h.db.CreateSession(ctx, session); err != nil {
		WriteError(w, fmt.Errorf("%w: failed to create session: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := CreateSessionResponse{
		ID:        sessionID,
		Status:    "pending",
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
	}

	// Add network info if ports are specified
	if len(req.Ports) > 0 && req.Network != "" && req.Network != "none" {
		response.Network = buildNetworkInfo(req.Network, ports, machine)
	}

	WriteJSON(w, response, http.StatusCreated)
}

// GetSession handles GET /v1/sessions/{id}
// Retrieves a session by ID, checking ownership.
func (h *Handlers) GetSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Get session ID from path
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		WriteError(w, fmt.Errorf("%w: session ID is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Get session from database
	session, err := h.db.GetSession(ctx, sessionID)
	if err != nil {
		WriteError(w, ErrNotFound, http.StatusNotFound, CodeNotFound)
		return
	}

	// Check ownership
	if session.APIKeyID != apiKeyID {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Build response
	response := buildSessionResponse(session)
	WriteJSON(w, response, http.StatusOK)
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

	WriteJSON(w, response, http.StatusOK)
}

// StopSession handles POST /v1/sessions/{id}/stop
// Stops a running session by stopping the Fly machine and updating the database.
func (h *Handlers) StopSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Get session ID from path
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		WriteError(w, fmt.Errorf("%w: session ID is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Get session from database
	session, err := h.db.GetSession(ctx, sessionID)
	if err != nil {
		WriteError(w, ErrNotFound, http.StatusNotFound, CodeNotFound)
		return
	}

	// Check ownership
	if session.APIKeyID != apiKeyID {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Check if already stopped
	if session.Status == "stopped" || session.Status == "killed" || session.Status == "failed" {
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
	status := "stopped"
	update := &db.SessionUpdate{
		Status:  &status,
		EndedAt: &now,
	}

	if err := h.db.UpdateSession(ctx, sessionID, update); err != nil {
		WriteError(w, fmt.Errorf("%w: failed to update session: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := StopSessionResponse{
		Status: "stopped",
	}

	WriteJSON(w, response, http.StatusOK)
}

// KillSession handles DELETE /v1/sessions/{id}
// Permanently deletes a session by destroying the Fly machine and updating the database.
func (h *Handlers) KillSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get API key ID from context
	apiKeyID, ok := GetAPIKeyID(ctx)
	if !ok {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
	}

	// Get session ID from path
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		WriteError(w, fmt.Errorf("%w: session ID is required", ErrBadRequest), http.StatusBadRequest, CodeBadRequest)
		return
	}

	// Get session from database
	session, err := h.db.GetSession(ctx, sessionID)
	if err != nil {
		WriteError(w, ErrNotFound, http.StatusNotFound, CodeNotFound)
		return
	}

	// Check ownership
	if session.APIKeyID != apiKeyID {
		WriteError(w, ErrUnauthorized, http.StatusUnauthorized, CodeUnauthorized)
		return
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
	status := "killed"
	update := &db.SessionUpdate{
		Status:  &status,
		EndedAt: &now,
	}

	if err := h.db.UpdateSession(ctx, sessionID, update); err != nil {
		WriteError(w, fmt.Errorf("%w: failed to update session: %v", ErrInternal, err), http.StatusInternalServerError, CodeInternal)
		return
	}

	// Build response
	response := StopSessionResponse{
		Status: "killed",
	}

	WriteJSON(w, response, http.StatusOK)
}

// generateSessionID creates a random session ID with the format "sess_<random>"
func generateSessionID() string {
	return fmt.Sprintf("sess_%s", randHex(12))
}

// randHex generates a random hex string of the specified length
func randHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID in case of error
		return fmt.Sprintf("%d", time.Now().UnixNano())
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
