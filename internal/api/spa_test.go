package api

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed testdata/spa/*
var testSPAFS embed.FS

func TestSPAHandler_ServeIndex(t *testing.T) {
	testFS, _ := fs.Sub(testSPAFS, "testdata/spa")
	handler := NewSPAHandler(testFS, "index.html", "/assets/")

	tests := []struct {
		name            string
		path            string
		wantStatusCode  int
		wantContains    string
		wantContentType string
	}{
		{
			name:            "root path serves index.html",
			path:            "/",
			wantStatusCode:  http.StatusOK,
			wantContains:    "<!DOCTYPE html>",
			wantContentType: "text/html",
		},
		{
			name:            "non-existent path serves index.html for SPA routing",
			path:            "/dashboard",
			wantStatusCode:  http.StatusOK,
			wantContains:    "<!DOCTYPE html>",
			wantContentType: "text/html",
		},
		{
			name:            "nested non-existent path serves index.html",
			path:            "/dashboard/sessions/123",
			wantStatusCode:  http.StatusOK,
			wantContains:    "<!DOCTYPE html>",
			wantContentType: "text/html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", resp.StatusCode, tt.wantStatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), tt.wantContains) {
				t.Errorf("body does not contain %q, got: %s", tt.wantContains, string(body))
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, tt.wantContentType) {
				t.Errorf("Content-Type = %q, want to contain %q", contentType, tt.wantContentType)
			}
		})
	}
}

func TestSPAHandler_ServeAssets(t *testing.T) {
	testFS, _ := fs.Sub(testSPAFS, "testdata/spa")
	handler := NewSPAHandler(testFS, "index.html", "/assets/")

	tests := []struct {
		name            string
		path            string
		wantStatusCode  int
		wantContains    string
		wantContentType string
		wantCacheHeader string
	}{
		{
			name:            "CSS file with correct content-type",
			path:            "/assets/main.css",
			wantStatusCode:  http.StatusOK,
			wantContains:    "body {",
			wantContentType: "text/css",
			wantCacheHeader: "immutable",
		},
		{
			name:            "JS file with correct content-type",
			path:            "/assets/app.js",
			wantStatusCode:  http.StatusOK,
			wantContains:    "console.log",
			wantContentType: "text/javascript",
			wantCacheHeader: "immutable",
		},
		{
			name:            "static file (not in assets) has shorter cache",
			path:            "/favicon.ico",
			wantStatusCode:  http.StatusOK,
			wantContentType: "image/",
			wantCacheHeader: "max-age=3600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", resp.StatusCode, tt.wantStatusCode)
			}

			if tt.wantContains != "" {
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), tt.wantContains) {
					t.Errorf("body does not contain %q, got: %s", tt.wantContains, string(body))
				}
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, tt.wantContentType) {
				t.Errorf("Content-Type = %q, want to contain %q", contentType, tt.wantContentType)
			}

			cacheControl := resp.Header.Get("Cache-Control")
			if !strings.Contains(cacheControl, tt.wantCacheHeader) {
				t.Errorf("Cache-Control = %q, want to contain %q", cacheControl, tt.wantCacheHeader)
			}
		})
	}
}

func TestSPAHandler_CacheHeaders(t *testing.T) {
	testFS, _ := fs.Sub(testSPAFS, "testdata/spa")
	handler := NewSPAHandler(testFS, "index.html", "/assets/")

	tests := []struct {
		name          string
		path          string
		wantImmutable bool
	}{
		{
			name:          "assets have immutable cache",
			path:          "/assets/main.css",
			wantImmutable: true,
		},
		{
			name:          "index.html does not have immutable cache",
			path:          "/",
			wantImmutable: false,
		},
		{
			name:          "non-asset files do not have immutable cache",
			path:          "/favicon.ico",
			wantImmutable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			cacheControl := resp.Header.Get("Cache-Control")
			hasImmutable := strings.Contains(cacheControl, "immutable")

			if hasImmutable != tt.wantImmutable {
				t.Errorf("Cache-Control immutable = %v, want %v (header: %q)",
					hasImmutable, tt.wantImmutable, cacheControl)
			}
		})
	}
}

func TestSPAHandler_FallbackMode(t *testing.T) {
	// Test with os.DirFS for dev mode
	testFS, _ := fs.Sub(testSPAFS, "testdata/spa")

	// Verify that the handler can be created with a filesystem
	handler := NewSPAHandler(testFS, "index.html", "/assets/")

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if handler.fs == nil {
		t.Error("expected non-nil filesystem")
	}
}
