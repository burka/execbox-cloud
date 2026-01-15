package fly

import (
	"testing"

	"github.com/burka/execbox/pkg/execbox"
)

// TestBackendImplementsInterface verifies Backend implements execbox.Backend at compile time.
func TestBackendImplementsInterface(t *testing.T) {
	var _ execbox.Backend = (*Backend)(nil)
	t.Log("Backend implements execbox.Backend")
}

// TestHandleImplementsInterface verifies Handle implements execbox.Handle at compile time.
func TestHandleImplementsInterface(t *testing.T) {
	var _ execbox.Handle = (*Handle)(nil)
	t.Log("Handle implements execbox.Handle")
}
