// Package api provides HTTP API types and error handling for execbox-cloud.
//
// This package implements the types defined in the Execbox Remote Protocol Specification.
// All types are designed to be JSON-compatible and match the protocol exactly for
// client/server compatibility.
//
// # Request/Response Types
//
// Session Management:
//   - CreateSessionRequest/CreateSessionResponse - POST /v1/sessions
//   - SessionResponse - GET /v1/sessions/{id}
//   - ListSessionsResponse - GET /v1/sessions
//   - StopSessionResponse - POST /v1/sessions/{id}/stop and DELETE /v1/sessions/{id}
//   - GetURLResponse - GET /v1/sessions/{id}/url
//
// File Operations:
//   - UploadFileResponse - POST /v1/sessions/{id}/files
//   - ListDirectoryResponse - GET /v1/sessions/{id}/files?list=true
//
// # Error Handling
//
// The package provides standard error types and helper functions:
//   - WriteError - Write a JSON error response
//   - WriteJSON - Write a JSON success response
//   - WriteErrorFromStandard - Map standard errors to HTTP responses
//
// Standard errors:
//   - ErrNotFound (404 NOT_FOUND)
//   - ErrUnauthorized (401 UNAUTHORIZED)
//   - ErrBadRequest (400 BAD_REQUEST)
//   - ErrConflict (409 CONFLICT)
//
// # Example Usage
//
//	func CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
//	    var req api.CreateSessionRequest
//	    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//	        api.WriteError(w, err, http.StatusBadRequest, api.CodeBadRequest)
//	        return
//	    }
//
//	    // Create session...
//	    resp := api.CreateSessionResponse{
//	        ID:        "sess_abc123",
//	        Status:    "running",
//	        CreatedAt: time.Now().UTC().Format(time.RFC3339),
//	    }
//
//	    api.WriteJSON(w, resp, http.StatusCreated)
//	}
package api

import "github.com/danielgtaylor/huma/v2"

// OpenAPIInfo returns the OpenAPI info configuration for execbox-cloud.
func OpenAPIInfo() huma.OpenAPI {
	return huma.OpenAPI{
		OpenAPI: "3.1.0",
		Info: &huma.Info{
			Title:       "Execbox Cloud API",
			Version:     "1.0.0",
			Description: "Remote execution API for AI assistants and automation.\n\nExecute code in secure cloud containers with full I/O streaming support via Fly.io infrastructure.",
			Contact: &huma.Contact{
				Name: "Execbox Cloud",
				URL:  "https://github.com/burka/execbox-cloud",
			},
		},
		Servers: []*huma.Server{
			{
				URL:         "https://api.execbox.cloud",
				Description: "Production server",
			},
			{
				URL:         "http://localhost:28080",
				Description: "Local development server",
			},
		},
		Tags: []*huma.Tag{
			{
				Name:        "Sessions",
				Description: "Create, manage, and monitor execution sessions",
			},
			{
				Name:        "Quota",
				Description: "Quota requests for increased limits",
			},
			{
				Name:        "Health",
				Description: "Health check endpoints",
			},
		},
	}
}

// SecuritySchemes returns the security scheme definitions.
func SecuritySchemes() map[string]*huma.SecurityScheme {
	return map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type:        "http",
			Scheme:      "bearer",
			Description: "API key authentication. Provide your API key in the Authorization header as 'Bearer YOUR_API_KEY'.",
		},
	}
}
