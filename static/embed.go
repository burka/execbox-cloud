//go:build !dev

package static

import (
	"embed"
	"io/fs"
)

// dashboardDistFS embeds the built dashboard SPA for production builds.
// The path is relative to this package at the module root.
// The dist directory should be copied to static/dist before building.
//
//go:embed all:dist
var dashboardDistFS embed.FS

// DashboardFS returns the embedded dashboard filesystem.
// The returned FS has the "dist" prefix stripped.
func DashboardFS() (fs.FS, error) {
	return fs.Sub(dashboardDistFS, "dist")
}
