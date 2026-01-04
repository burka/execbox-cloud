// Package fly provides a client for the Fly.io Machines API
package fly

import "fmt"

// FlyError represents an error returned by the Fly.io API
type FlyError struct {
	StatusCode int
	Message    string
}

// Error implements the error interface
func (e *FlyError) Error() string {
	return fmt.Sprintf("fly api error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error represents a 404 Not Found response
func (e *FlyError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsRateLimited returns true if the error represents a 429 Too Many Requests response
func (e *FlyError) IsRateLimited() bool {
	return e.StatusCode == 429
}
