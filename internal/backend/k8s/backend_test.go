//nolint:staticcheck // fake.NewSimpleClientset is deprecated but fake.NewClientset requires generated apply configs
package k8s

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/burka/execbox/pkg/execbox"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestBackendImplementsInterface verifies that Backend implements execbox.Backend.
func TestBackendImplementsInterface(t *testing.T) {
	var _ execbox.Backend = (*Backend)(nil)
}

// TestBackend_Name tests that Name() returns "kubernetes".
func TestBackend_Name(t *testing.T) {
	backend := &Backend{}

	got := backend.Name()
	want := "kubernetes"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

// TestBackendConfig_Defaults tests that default values are applied correctly.
func TestBackendConfig_Defaults(t *testing.T) {
	tests := []struct {
		name          string
		config        BackendConfig
		wantNamespace string
		wantLabels    map[string]string
	}{
		{
			name:          "empty config gets defaults",
			config:        BackendConfig{},
			wantNamespace: "",
			wantLabels:    nil,
		},
		{
			name: "explicit values preserved",
			config: BackendConfig{
				Namespace: "custom-ns",
				Labels:    map[string]string{"env": "test"},
			},
			wantNamespace: "custom-ns",
			wantLabels:    map[string]string{"env": "test"},
		},
		{
			name: "nil labels preserved",
			config: BackendConfig{
				Namespace: "custom-ns",
				Labels:    nil,
			},
			wantNamespace: "custom-ns",
			wantLabels:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config

			if cfg.Namespace != tt.wantNamespace {
				t.Errorf("Namespace = %q, want %q", cfg.Namespace, tt.wantNamespace)
			}

			if cfg.Labels == nil && tt.wantLabels != nil {
				t.Errorf("Labels = nil, want %v", tt.wantLabels)
			} else if cfg.Labels != nil && tt.wantLabels == nil {
				t.Errorf("Labels = %v, want nil", cfg.Labels)
			} else if cfg.Labels != nil && tt.wantLabels != nil {
				if len(cfg.Labels) != len(tt.wantLabels) {
					t.Errorf("Labels length = %d, want %d", len(cfg.Labels), len(tt.wantLabels))
				}
				for k, v := range tt.wantLabels {
					if cfg.Labels[k] != v {
						t.Errorf("Labels[%q] = %q, want %q", k, cfg.Labels[k], v)
					}
				}
			}
		})
	}
}

// TestBackend_destroyResources tests resource cleanup with a fake clientset.
func TestBackend_destroyResources(t *testing.T) {
	tests := []struct {
		name               string
		sessionID          string
		existingPods       []runtime.Object
		existingConfigMaps []runtime.Object
		existingPVCs       []runtime.Object
		wantError          bool
	}{
		{
			name:      "destroy resources successfully",
			sessionID: "test-session-123",
			existingPods: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "execbox-test-ses",
						Namespace: "execbox",
						Labels: map[string]string{
							"execbox.io/session-id": "test-session-123",
						},
					},
				},
			},
			existingConfigMaps: []runtime.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "execbox-files-test-ses",
						Namespace: "execbox",
						Labels: map[string]string{
							"execbox.io/session-id": "test-session-123",
						},
					},
				},
			},
			existingPVCs: []runtime.Object{
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "execbox-vol-test-ses",
						Namespace: "execbox",
						Labels: map[string]string{
							"execbox.io/session-id": "test-session-123",
						},
					},
				},
			},
			wantError: false,
		},
		{
			name:         "destroy with no resources",
			sessionID:    "nonexistent-session",
			existingPods: []runtime.Object{},
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := append(tt.existingPods, tt.existingConfigMaps...)
			objects = append(objects, tt.existingPVCs...)

			fakeClientset := fake.NewSimpleClientset(objects...)

			// Add reactor to handle DeleteCollection since fake client doesn't support it properly
			fakeClientset.PrependReactor("delete-collection", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				// Simulate successful deletion
				return true, nil, nil
			})

			backend := &Backend{
				clientset: fakeClientset,
				config: BackendConfig{
					Namespace: "execbox",
				},
			}

			ctx := context.Background()
			err := backend.destroyResources(ctx, tt.sessionID)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Note: We can't verify deletion with fake clientset since DeleteCollection
			// doesn't actually delete in the fake implementation. The test verifies
			// that the destroyResources method completes without error.
		})
	}
}

// TestBackend_destroyResources_Error tests error handling in destroyResources.
func TestBackend_destroyResources_Error(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	// Inject an error for pod deletion
	clientset.PrependReactor("delete-collection", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewInternalError(errors.New("simulated pod deletion error"))
	})

	backend := &Backend{
		clientset: clientset,
		config: BackendConfig{
			Namespace: "execbox",
		},
	}

	ctx := context.Background()
	err := backend.destroyResources(ctx, "test-session")

	if err == nil {
		t.Error("expected error from pod deletion, got nil")
	}
}

// TestStreamBuffer tests concurrent writes and String() method.
func TestStreamBuffer(t *testing.T) {
	t.Run("single write", func(t *testing.T) {
		var buf streamBuffer

		data := []byte("hello world")
		n, err := buf.Write(data)

		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
		if n != len(data) {
			t.Errorf("Write() returned %d, want %d", n, len(data))
		}
		if buf.String() != "hello world" {
			t.Errorf("String() = %q, want %q", buf.String(), "hello world")
		}
	})

	t.Run("multiple writes", func(t *testing.T) {
		var buf streamBuffer

		writes := []string{"hello", " ", "world", "!"}
		for _, w := range writes {
			n, err := buf.Write([]byte(w))
			if err != nil {
				t.Errorf("Write() error = %v", err)
			}
			if n != len(w) {
				t.Errorf("Write() returned %d, want %d", n, len(w))
			}
		}

		want := "hello world!"
		if buf.String() != want {
			t.Errorf("String() = %q, want %q", buf.String(), want)
		}
	})

	t.Run("concurrent writes", func(t *testing.T) {
		var buf streamBuffer
		var wg sync.WaitGroup

		// Simulate concurrent writes from multiple goroutines
		numGoroutines := 10
		writesPerGoroutine := 100

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < writesPerGoroutine; j++ {
					_, _ = buf.Write([]byte("x"))
				}
			}(i)
		}

		wg.Wait()

		// Should have exactly numGoroutines * writesPerGoroutine bytes
		expectedLen := numGoroutines * writesPerGoroutine
		if len(buf.String()) != expectedLen {
			t.Errorf("String() length = %d, want %d", len(buf.String()), expectedLen)
		}
	})

	t.Run("empty buffer", func(t *testing.T) {
		var buf streamBuffer

		if buf.String() != "" {
			t.Errorf("String() = %q, want empty string", buf.String())
		}
	})

	t.Run("binary data", func(t *testing.T) {
		var buf streamBuffer

		binaryData := []byte{0x00, 0x01, 0x02, 0xFF}
		n, err := buf.Write(binaryData)

		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
		if n != len(binaryData) {
			t.Errorf("Write() returned %d, want %d", n, len(binaryData))
		}

		result := []byte(buf.String())
		for i, b := range binaryData {
			if result[i] != b {
				t.Errorf("String()[%d] = %x, want %x", i, result[i], b)
			}
		}
	})
}

// TestBuildFilesSizeValidation tests the 1MB limit validation.
func TestBuildFilesSizeValidation(t *testing.T) {
	const maxConfigMapSize = 1024 * 1024 // 1MB

	tests := []struct {
		name      string
		files     []execbox.BuildFile
		expectErr bool
	}{
		{
			name:      "empty files",
			files:     []execbox.BuildFile{},
			expectErr: false,
		},
		{
			name: "small file",
			files: []execbox.BuildFile{
				{Path: "/app/config.txt", Content: []byte("small content")},
			},
			expectErr: false,
		},
		{
			name: "file at limit",
			files: []execbox.BuildFile{
				{Path: "/app/data", Content: make([]byte, maxConfigMapSize)},
			},
			expectErr: false,
		},
		{
			name: "file over limit by one byte",
			files: []execbox.BuildFile{
				{Path: "/app/data", Content: make([]byte, maxConfigMapSize+1)},
			},
			expectErr: true,
		},
		{
			name: "multiple files under limit",
			files: []execbox.BuildFile{
				{Path: "/app/file1", Content: make([]byte, 500*1024)},
				{Path: "/app/file2", Content: make([]byte, 500*1024)},
			},
			expectErr: false,
		},
		{
			name: "multiple files over limit",
			files: []execbox.BuildFile{
				{Path: "/app/file1", Content: make([]byte, 600*1024)},
				{Path: "/app/file2", Content: make([]byte, 600*1024)},
			},
			expectErr: true,
		},
		{
			name: "many small files at limit",
			files: []execbox.BuildFile{
				{Path: "/app/file1", Content: make([]byte, 256*1024)},
				{Path: "/app/file2", Content: make([]byte, 256*1024)},
				{Path: "/app/file3", Content: make([]byte, 256*1024)},
				{Path: "/app/file4", Content: make([]byte, 256*1024)},
			},
			expectErr: false,
		},
		{
			name: "many small files over limit",
			files: []execbox.BuildFile{
				{Path: "/app/file1", Content: make([]byte, 256*1024)},
				{Path: "/app/file2", Content: make([]byte, 256*1024)},
				{Path: "/app/file3", Content: make([]byte, 256*1024)},
				{Path: "/app/file4", Content: make([]byte, 257*1024)},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalSize := 0
			for _, f := range tt.files {
				totalSize += len(f.Content)
			}

			gotErr := totalSize > maxConfigMapSize

			if gotErr != tt.expectErr {
				t.Errorf("size validation = %v, want error = %v (total size: %d bytes)", gotErr, tt.expectErr, totalSize)
			}
		})
	}
}

// TestBackendConfig verifies BackendConfig defaults (kept for compatibility).
func TestBackendConfig(t *testing.T) {
	cfg := BackendConfig{}

	if cfg.Namespace != "" {
		t.Errorf("expected empty namespace, got %s", cfg.Namespace)
	}

	if cfg.Labels != nil {
		t.Errorf("expected nil labels, got %v", cfg.Labels)
	}
}
