package k8s

import (
	"testing"

	"github.com/burka/execbox/pkg/execbox"
)

// TestBackendImplementsInterface verifies that Backend implements execbox.Backend.
func TestBackendImplementsInterface(t *testing.T) {
	var _ execbox.Backend = (*Backend)(nil)
}

// TestBackendConfig verifies BackendConfig defaults.
func TestBackendConfig(t *testing.T) {
	cfg := BackendConfig{}

	if cfg.Namespace != "" {
		t.Errorf("expected empty namespace, got %s", cfg.Namespace)
	}

	if cfg.Labels != nil {
		t.Errorf("expected nil labels, got %v", cfg.Labels)
	}
}
