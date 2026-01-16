package api

import (
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
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

// GenerateOpenAPISpec generates the OpenAPI specification without creating a full server.
// This is useful for SDK generation and documentation without requiring database connection.
// It uses the same RegisterRoutes function as the runtime server, ensuring the spec
// is always in sync with the actual API implementation (single source of truth).
func GenerateOpenAPISpec() ([]byte, error) {
	router := chi.NewRouter()

	// Create stub services - handlers won't be called during spec generation,
	// huma only needs the function signatures to extract request/response types.
	stubServices := &Services{
		Session: NewSessionService(nil, nil),
		Account: NewAccountService(nil),
		Quota:   NewQuotaService(nil),
		DB:      nil, // nil DB signals spec-generation mode to RegisterRoutes
	}

	// Use the same route registration as the runtime server.
	// This ensures the OpenAPI spec exactly matches what the server implements.
	humaAPI := RegisterRoutes(router, stubServices, nil)

	return humaAPI.OpenAPI().MarshalJSON()
}
