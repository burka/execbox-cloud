package api

import (
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// SPAHandler serves a Single Page Application from a filesystem.
// It handles SPA routing by serving index.html for non-file routes and sets
// appropriate cache headers for static assets.
type SPAHandler struct {
	fs              fs.FS
	indexPath       string
	assetsPrefix    string
	maxAge          int
	immutableMaxAge int
}

// NewSPAHandler creates a new SPA handler.
//
// Parameters:
//   - fileSystem: The filesystem to serve from (embedded or os.DirFS)
//   - indexPath: Path to index.html within the filesystem (e.g., "index.html")
//   - assetsPrefix: URL prefix for static assets that should be cached (e.g., "/assets/")
func NewSPAHandler(fileSystem fs.FS, indexPath, assetsPrefix string) *SPAHandler {
	return &SPAHandler{
		fs:              fileSystem,
		indexPath:       indexPath,
		assetsPrefix:    assetsPrefix,
		maxAge:          3600,     // 1 hour for index.html
		immutableMaxAge: 31536000, // 1 year for hashed assets
	}
}

// ServeHTTP implements http.Handler.
func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the URL path
	urlPath := path.Clean(r.URL.Path)

	// Remove leading slash for filesystem access
	fsPath := strings.TrimPrefix(urlPath, "/")
	if fsPath == "" {
		fsPath = "."
	}

	// Try to open the requested file
	file, err := h.fs.Open(fsPath)
	if err == nil {
		defer file.Close()

		// Check if it's a file (not a directory)
		stat, err := file.Stat()
		if err == nil && !stat.IsDir() {
			// Set cache headers based on path
			h.setCacheHeaders(w, urlPath)

			// Set content type
			h.setContentType(w, fsPath)

			// Serve the file using http.FileServer compatibility
			if seeker, ok := file.(interface {
				fs.File
				http.File
			}); ok {
				http.ServeContent(w, r, fsPath, stat.ModTime(), seeker)
			} else {
				// Fallback: read entire file
				content, err := io.ReadAll(file)
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				http.ServeContent(w, r, fsPath, stat.ModTime(), strings.NewReader(string(content)))
			}
			return
		}
		file.Close()
	}

	// File not found or is a directory - serve index.html for SPA routing
	h.serveIndex(w, r)
}

// serveIndex serves the index.html file for SPA routing.
func (h *SPAHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	file, err := h.fs.Open(h.indexPath)
	if err != nil {
		slog.Error("failed to open index.html", "error", err, "path", h.indexPath)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		slog.Error("failed to stat index.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set cache headers for index.html (shorter cache)
	w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Serve using http.FileServer compatibility
	if seeker, ok := file.(interface {
		fs.File
		http.File
	}); ok {
		http.ServeContent(w, r, h.indexPath, stat.ModTime(), seeker)
	} else {
		// Fallback: read entire file
		content, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, h.indexPath, stat.ModTime(), strings.NewReader(string(content)))
	}
}

// setCacheHeaders sets appropriate cache headers based on the request path.
func (h *SPAHandler) setCacheHeaders(w http.ResponseWriter, urlPath string) {
	if strings.HasPrefix(urlPath, h.assetsPrefix) {
		// Static assets with hash in filename - cache aggressively
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// Other files (like index.html) - shorter cache with revalidation
		w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
	}
}

// setContentType sets the Content-Type header based on file extension.
func (h *SPAHandler) setContentType(w http.ResponseWriter, filePath string) {
	ext := filepath.Ext(filePath)
	if ext != "" {
		contentType := mime.TypeByExtension(ext)
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
	}
}
