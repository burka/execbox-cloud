package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Standard API errors
var (
	ErrNotFound      = errors.New("session not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrBadRequest    = errors.New("bad request")
	ErrConflict      = errors.New("session already stopped")
	ErrInternal      = errors.New("internal server error")
	ErrQuotaExceeded = errors.New("quota exceeded")
)

// ErrorResponse defines the standard error response format
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Error code constants matching the spec
const (
	CodeBadRequest     = "BAD_REQUEST"
	CodeUnauthorized   = "UNAUTHORIZED"
	CodeNotFound       = "NOT_FOUND"
	CodeConflict       = "CONFLICT"
	CodeInternal       = "INTERNAL"
	CodeQuotaExceeded  = "QUOTA_EXCEEDED"
	CodeNotImplemented = "NOT_IMPLEMENTED"
)

// WriteError writes a JSON error response to the HTTP response writer
func WriteError(w http.ResponseWriter, err error, statusCode int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error: err.Error(),
		Code:  code,
	}

	// Ignore encoding errors - nothing we can do at this point
	_ = json.NewEncoder(w).Encode(response)
}

// WriteJSON writes a JSON response to the HTTP response writer
func WriteJSON(w http.ResponseWriter, data interface{}, statusCode int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	return json.NewEncoder(w).Encode(data)
}

// WriteErrorFromStandard is a helper that maps standard errors to HTTP status codes
func WriteErrorFromStandard(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		WriteError(w, err, http.StatusNotFound, CodeNotFound)
	case errors.Is(err, ErrUnauthorized):
		WriteError(w, err, http.StatusUnauthorized, CodeUnauthorized)
	case errors.Is(err, ErrBadRequest):
		WriteError(w, err, http.StatusBadRequest, CodeBadRequest)
	case errors.Is(err, ErrConflict):
		WriteError(w, err, http.StatusConflict, CodeConflict)
	default:
		WriteError(w, err, http.StatusInternalServerError, CodeInternal)
	}
}
