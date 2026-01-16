package api

import (
	"encoding/json"
	"testing"
)

// TestGenerateOpenAPISpec verifies that the OpenAPI spec can be generated without errors.
func TestGenerateOpenAPISpec(t *testing.T) {
	spec, err := GenerateOpenAPISpec()
	if err != nil {
		t.Fatalf("GenerateOpenAPISpec() failed: %v", err)
	}

	if len(spec) == 0 {
		t.Fatal("GenerateOpenAPISpec() returned empty spec")
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(spec, &result); err != nil {
		t.Fatalf("Generated spec is not valid JSON: %v", err)
	}

	// Verify required OpenAPI fields
	if _, ok := result["openapi"]; !ok {
		t.Error("Generated spec missing 'openapi' field")
	}

	if _, ok := result["info"]; !ok {
		t.Error("Generated spec missing 'info' field")
	}

	if _, ok := result["paths"]; !ok {
		t.Error("Generated spec missing 'paths' field")
	}
}

// TestOpenAPIInfo verifies the OpenAPI metadata is correctly configured.
func TestOpenAPIInfo(t *testing.T) {
	info := OpenAPIInfo()

	if info.OpenAPI != "3.1.0" {
		t.Errorf("Expected OpenAPI version 3.1.0, got %s", info.OpenAPI)
	}

	if info.Info == nil {
		t.Fatal("Info is nil")
	}

	if info.Info.Title != "Execbox Cloud API" {
		t.Errorf("Expected title 'Execbox Cloud API', got %s", info.Info.Title)
	}

	if info.Info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", info.Info.Version)
	}

	// Verify tags
	if len(info.Tags) == 0 {
		t.Error("Expected at least one tag")
	}

	expectedTags := map[string]bool{
		"Sessions": false,
		"Quota":    false,
		"Health":   false,
	}

	for _, tag := range info.Tags {
		if _, ok := expectedTags[tag.Name]; ok {
			expectedTags[tag.Name] = true
		}
	}

	for name, found := range expectedTags {
		if !found {
			t.Errorf("Expected tag '%s' not found", name)
		}
	}
}

// TestSecuritySchemes verifies security scheme configuration.
func TestSecuritySchemes(t *testing.T) {
	schemes := SecuritySchemes()

	if len(schemes) == 0 {
		t.Fatal("Expected at least one security scheme")
	}

	bearerAuth, ok := schemes["bearerAuth"]
	if !ok {
		t.Fatal("Expected 'bearerAuth' security scheme")
	}

	if bearerAuth.Type != "http" {
		t.Errorf("Expected type 'http', got %s", bearerAuth.Type)
	}

	if bearerAuth.Scheme != "bearer" {
		t.Errorf("Expected scheme 'bearer', got %s", bearerAuth.Scheme)
	}
}

// TestOpenAPIHasAllEndpoints verifies that all expected endpoints are documented.
func TestOpenAPIHasAllEndpoints(t *testing.T) {
	spec, err := GenerateOpenAPISpec()
	if err != nil {
		t.Fatalf("GenerateOpenAPISpec() failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(spec, &result); err != nil {
		t.Fatalf("Failed to unmarshal spec: %v", err)
	}

	paths, ok := result["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("'paths' is not a map")
	}

	// Expected endpoints
	// Note: /v1/sessions/{id}/attach is handled via chi directly (WebSocket upgrade)
	// and is not registered through huma, so it won't appear in the OpenAPI spec.
	expectedEndpoints := []string{
		"/health",
		"/v1/sessions",
		"/v1/sessions/{id}",
		"/v1/sessions/{id}/stop",
		"/v1/quota-requests",
	}

	for _, endpoint := range expectedEndpoints {
		if _, ok := paths[endpoint]; !ok {
			t.Errorf("Expected endpoint '%s' not found in OpenAPI spec", endpoint)
		}
	}
}
