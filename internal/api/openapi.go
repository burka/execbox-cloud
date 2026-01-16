package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// handleOpenAPIJSON serves the OpenAPI spec as JSON.
func handleOpenAPIJSON(api huma.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if api == nil {
			http.Error(w, "OpenAPI spec not available", http.StatusServiceUnavailable)
			return
		}
		spec, err := api.OpenAPI().MarshalJSON()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to generate OpenAPI spec: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(spec)
	}
}

// handleOpenAPIYAML serves the OpenAPI spec as YAML.
func handleOpenAPIYAML(api huma.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if api == nil {
			http.Error(w, "OpenAPI spec not available", http.StatusServiceUnavailable)
			return
		}
		spec := api.OpenAPI()
		yamlBytes, err := yaml.Marshal(spec)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to generate OpenAPI spec: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(yamlBytes)
	}
}

// RegisterHumaRoutes registers API routes with huma for OpenAPI documentation.
// The actual request handling is done by the chi handlers registered separately.
// This registration is purely for OpenAPI schema generation.
func RegisterHumaRoutes(humaAPI huma.API) {
	// Health endpoint
	huma.Register(humaAPI, huma.Operation{
		OperationID: "health",
		Method:      "GET",
		Path:        "/health",
		Summary:     "Health check",
		Description: "Returns server health status. Does not require authentication.",
		Tags:        []string{"Health"},
	}, func(ctx context.Context, input *HealthCheckInput) (*HealthCheckOutput, error) {
		return nil, nil
	})

	// Session operations
	huma.Register(humaAPI, huma.Operation{
		OperationID:   "createSession",
		Method:        "POST",
		Path:          "/v1/sessions",
		Summary:       "Create a new session",
		Description:   "Create a new execution session with the specified container image and configuration.",
		Tags:          []string{"Sessions"},
		Security:      []map[string][]string{{"bearerAuth": {}}},
		DefaultStatus: 201,
	}, func(ctx context.Context, input *CreateSessionInput) (*CreateSessionOutput, error) {
		return nil, nil
	})

	huma.Register(humaAPI, huma.Operation{
		OperationID: "listSessions",
		Method:      "GET",
		Path:        "/v1/sessions",
		Summary:     "List all sessions",
		Description: "Returns a list of all active and recently completed sessions for the authenticated user.",
		Tags:        []string{"Sessions"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *ListSessionsInput) (*ListSessionsOutput, error) {
		return nil, nil
	})

	huma.Register(humaAPI, huma.Operation{
		OperationID: "getSession",
		Method:      "GET",
		Path:        "/v1/sessions/{id}",
		Summary:     "Get session info",
		Description: "Returns detailed information about a specific session.",
		Tags:        []string{"Sessions"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *GetSessionInput) (*GetSessionOutput, error) {
		return nil, nil
	})

	huma.Register(humaAPI, huma.Operation{
		OperationID: "stopSession",
		Method:      "POST",
		Path:        "/v1/sessions/{id}/stop",
		Summary:     "Stop a session",
		Description: "Gracefully stop a running session.",
		Tags:        []string{"Sessions"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *StopSessionInput) (*StopSessionOutput, error) {
		return nil, nil
	})

	huma.Register(humaAPI, huma.Operation{
		OperationID:   "killSession",
		Method:        "DELETE",
		Path:          "/v1/sessions/{id}",
		Summary:       "Kill a session",
		Description:   "Forcefully terminate a session immediately.",
		Tags:          []string{"Sessions"},
		Security:      []map[string][]string{{"bearerAuth": {}}},
		DefaultStatus: 204,
	}, func(ctx context.Context, input *KillSessionInput) (*KillSessionOutput, error) {
		return nil, nil
	})

	// WebSocket endpoints (documentation only - actual handlers are chi routes)
	huma.Register(humaAPI, huma.Operation{
		OperationID: "attachSession",
		Method:      "GET",
		Path:        "/v1/sessions/{id}/attach",
		Summary:     "Attach to session I/O via WebSocket",
		Description: "Upgrade to WebSocket for bidirectional stdin/stdout/stderr streaming.\n\n**Protocol:** JSON messages with format `{\"type\": \"...\", \"data\": \"...\"}`\n\n**Client → Server:** `stdin` (base64 data), `stdinClose`, `resize` (cols/rows)\n\n**Server → Client:** `stdout`, `stderr` (base64 data), `exit` (code), `error` (msg)",
		Tags:        []string{"Sessions"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *AttachSessionInput) (*HealthCheckOutput, error) {
		return nil, nil
	})

	// Quota operations
	huma.Register(humaAPI, huma.Operation{
		OperationID:   "createQuotaRequest",
		Method:        "POST",
		Path:          "/v1/quota-requests",
		Summary:       "Request quota increase",
		Description:   "Submit a request to increase API usage limits. Does not require authentication.",
		Tags:          []string{"Quota"},
		DefaultStatus: 201,
	}, func(ctx context.Context, input *CreateQuotaRequestInput) (*CreateQuotaRequestOutput, error) {
		return nil, nil
	})

	// Account operations
	huma.Register(humaAPI, huma.Operation{
		OperationID: "getAccount",
		Method:      "GET",
		Path:        "/v1/account",
		Summary:     "Get account information",
		Description: "Returns account information including tier, email, and API key details.",
		Tags:        []string{"Account"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *GetAccountInput) (*GetAccountOutput, error) {
		return nil, nil
	})

	huma.Register(humaAPI, huma.Operation{
		OperationID: "getUsage",
		Method:      "GET",
		Path:        "/v1/account/usage",
		Summary:     "Get usage statistics",
		Description: "Returns usage statistics including sessions today, quota remaining, and limits.",
		Tags:        []string{"Account"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *GetUsageInput) (*GetUsageOutput, error) {
		return nil, nil
	})

	// API Key operations
	huma.Register(humaAPI, huma.Operation{
		OperationID:   "createAPIKey",
		Method:        "POST",
		Path:          "/v1/keys",
		Summary:       "Create a new API key",
		Description:   "Create a new API key for accessing the API. Does not require authentication.",
		Tags:          []string{"API Keys"},
		DefaultStatus: 201,
	}, func(ctx context.Context, input *CreateAPIKeyInput) (*CreateAPIKeyOutput, error) {
		return nil, nil
	})
}

// GenerateOpenAPISpec generates the OpenAPI specification without creating a full server.
// This is useful for SDK generation and documentation without requiring database connection.
func GenerateOpenAPISpec() ([]byte, error) {
	router := chi.NewRouter()

	config := huma.DefaultConfig("Execbox Cloud API", "1.0.0")
	config.Info = OpenAPIInfo().Info
	config.Tags = OpenAPIInfo().Tags
	config.Servers = OpenAPIInfo().Servers

	humaAPI := humachi.New(router, config)

	// Initialize Components.SecuritySchemes if nil
	if humaAPI.OpenAPI().Components.SecuritySchemes == nil {
		humaAPI.OpenAPI().Components.SecuritySchemes = make(map[string]*huma.SecurityScheme)
	}

	// Register security schemes
	for name, scheme := range SecuritySchemes() {
		humaAPI.OpenAPI().Components.SecuritySchemes[name] = scheme
	}

	// Register huma operations for OpenAPI documentation
	RegisterHumaRoutes(humaAPI)

	return humaAPI.OpenAPI().MarshalJSON()
}

// SetupOpenAPI initializes huma API on the given router and returns the huma API instance.
// It also registers the OpenAPI endpoints (/openapi.json and /openapi.yaml).
func SetupOpenAPI(router *chi.Mux) huma.API {
	// Set up huma API for OpenAPI documentation
	config := huma.DefaultConfig("Execbox Cloud API", "1.0.0")
	config.Info = OpenAPIInfo().Info
	config.Tags = OpenAPIInfo().Tags
	config.Servers = OpenAPIInfo().Servers
	humaAPI := humachi.New(router, config)

	// Initialize Components.SecuritySchemes if nil
	if humaAPI.OpenAPI().Components.SecuritySchemes == nil {
		humaAPI.OpenAPI().Components.SecuritySchemes = make(map[string]*huma.SecurityScheme)
	}

	// Register security schemes
	for name, scheme := range SecuritySchemes() {
		humaAPI.OpenAPI().Components.SecuritySchemes[name] = scheme
	}

	// Register huma operations for OpenAPI spec generation only.
	// We use a separate router to avoid route conflicts with the actual chi handlers.
	specRouter := chi.NewRouter()
	specConfig := huma.DefaultConfig("Execbox Cloud API", "1.0.0")
	specConfig.Info = OpenAPIInfo().Info
	specConfig.Tags = OpenAPIInfo().Tags
	specConfig.Servers = OpenAPIInfo().Servers
	specAPI := humachi.New(specRouter, specConfig)

	// Initialize Components.SecuritySchemes for spec API
	if specAPI.OpenAPI().Components.SecuritySchemes == nil {
		specAPI.OpenAPI().Components.SecuritySchemes = make(map[string]*huma.SecurityScheme)
	}
	for name, scheme := range SecuritySchemes() {
		specAPI.OpenAPI().Components.SecuritySchemes[name] = scheme
	}

	// Register huma routes on the spec-only router (not the main router)
	RegisterHumaRoutes(specAPI)

	// Register OpenAPI spec endpoints on the main router (unauthenticated)
	router.Get("/openapi.json", handleOpenAPIJSON(specAPI))
	router.Get("/openapi.yaml", handleOpenAPIYAML(specAPI))

	return humaAPI
}
