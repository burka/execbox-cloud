package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	err := errors.New("test error")

	WriteError(w, err, http.StatusNotFound, CodeNotFound)

	// Check status code
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != "test error" {
		t.Errorf("Expected error 'test error', got %s", resp.Error)
	}
	if resp.Code != CodeNotFound {
		t.Errorf("Expected code %s, got %s", CodeNotFound, resp.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	err := WriteJSON(w, data, http.StatusOK)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %s", resp["status"])
	}
}

func TestWriteErrorFromStandard(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "NotFound",
			err:            ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   CodeNotFound,
		},
		{
			name:           "Unauthorized",
			err:            ErrUnauthorized,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   CodeUnauthorized,
		},
		{
			name:           "BadRequest",
			err:            ErrBadRequest,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   CodeBadRequest,
		},
		{
			name:           "Conflict",
			err:            ErrConflict,
			expectedStatus: http.StatusConflict,
			expectedCode:   CodeConflict,
		},
		{
			name:           "Unknown",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteErrorFromStandard(w, tt.err)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response body
			var resp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if resp.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, resp.Code)
			}
		})
	}
}

func TestWriteErrorFromStandardWrapped(t *testing.T) {
	// Test that wrapped errors are handled correctly
	wrappedErr := errors.Join(ErrNotFound, errors.New("additional context"))

	w := httptest.NewRecorder()
	WriteErrorFromStandard(w, wrappedErr)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for wrapped ErrNotFound, got %d", http.StatusNotFound, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Code != CodeNotFound {
		t.Errorf("Expected code %s for wrapped error, got %s", CodeNotFound, resp.Code)
	}
}
