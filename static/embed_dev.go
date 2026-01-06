//go:build dev

package static

import (
	"io/fs"
	"os"
)

// DashboardFS returns the dashboard filesystem from the actual directory in dev mode.
// This allows for hot reloading during development.
func DashboardFS() (fs.FS, error) {
	return os.DirFS("dashboard/dist"), nil
}
