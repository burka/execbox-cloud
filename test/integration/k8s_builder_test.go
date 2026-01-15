//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/k8s"
	"github.com/burka/execbox/pkg/execbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func createK8sClientset(t *testing.T) kubernetes.Interface {
	t.Helper()

	config, err := clientcmd.BuildConfigFromFlags("", "/tmp/microk8s-kubeconfig")
	require.NoError(t, err, "Failed to load kubeconfig")

	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err, "Failed to create clientset")

	return clientset
}

func TestK8sBuilder_SimpleImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping builder test in short mode")
	}

	clientset := createK8sClientset(t)

	builder := k8s.NewBuilder(clientset, "execbox-test", k8s.BuilderConfig{
		Registry: "ttl.sh",
		ImageTTL: "1h",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Build a simple image with a setup command
	spec := k8s.BuildSpec{
		BaseImage: "alpine:latest",
		Setup:     []string{"echo 'build test' > /tmp/marker"},
	}

	imageRef, err := builder.Build(ctx, spec)
	require.NoError(t, err, "Build should succeed")

	t.Logf("✅ Built image: %s", imageRef)

	// Verify the image works by running it
	backend, err := k8s.NewBackend(k8s.BackendConfig{
		Kubeconfig: "/tmp/microk8s-kubeconfig",
		Namespace:  "execbox-test",
	})
	require.NoError(t, err)
	defer backend.Close()

	handle, err := backend.Run(ctx, execbox.Spec{
		Image:   imageRef,
		Command: []string{"cat", "/tmp/marker"},
	})
	require.NoError(t, err)
	defer backend.Destroy(ctx, handle.ID())

	// Wait for completion and check output
	result := <-handle.Wait()
	assert.Equal(t, 0, result.Code, "Exit code should be 0")

	// Read output
	stdout := handle.Stdout()
	buf := make([]byte, 1024)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "build test", "Output should contain marker from build")
	t.Logf("✅ Output from built image: %q", output)
}

func TestK8sBuilder_WithFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping builder test in short mode")
	}

	clientset := createK8sClientset(t)

	builder := k8s.NewBuilder(clientset, "execbox-test", k8s.BuilderConfig{
		Registry: "ttl.sh",
		ImageTTL: "1h",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Build image with a file baked in
	// Use unique content to avoid ttl.sh cache from previous broken builds
	spec := k8s.BuildSpec{
		BaseImage: "alpine:latest",
		Files: []execbox.BuildFile{
			{
				Path:    "/app/config.json",
				Content: []byte(`{"name": "test", "version": "2.0-fixed"}`),
			},
		},
	}

	imageRef, err := builder.Build(ctx, spec)
	require.NoError(t, err, "Build should succeed")

	t.Logf("✅ Built image with files: %s", imageRef)

	// Verify the file exists in the image
	backend, err := k8s.NewBackend(k8s.BackendConfig{
		Kubeconfig: "/tmp/microk8s-kubeconfig",
		Namespace:  "execbox-test",
	})
	require.NoError(t, err)
	defer backend.Close()

	handle, err := backend.Run(ctx, execbox.Spec{
		Image:   imageRef,
		Command: []string{"cat", "/app/config.json"},
	})
	require.NoError(t, err)
	defer backend.Destroy(ctx, handle.ID())

	result := <-handle.Wait()
	assert.Equal(t, 0, result.Code)

	stdout := handle.Stdout()
	buf := make([]byte, 1024)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, `"name": "test"`, "Output should contain config content")
	assert.Contains(t, output, `2.0-fixed`, "Output should contain version")
	t.Logf("✅ File content from built image: %q", output)
}

func TestK8sBuilder_ContentAddressedCaching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping builder test in short mode")
	}

	clientset := createK8sClientset(t)

	builder := k8s.NewBuilder(clientset, "execbox-test", k8s.BuilderConfig{
		Registry: "ttl.sh",
		ImageTTL: "1h",
	})

	ctx := context.Background()

	// Build the same spec twice
	spec := k8s.BuildSpec{
		BaseImage: "alpine:latest",
		Setup:     []string{"echo 'cache test'"},
	}

	imageRef1, err := builder.Build(ctx, spec)
	require.NoError(t, err)

	imageRef2, err := builder.Build(ctx, spec)
	require.NoError(t, err)

	// Same inputs should produce same tag (content-addressed)
	assert.Equal(t, imageRef1, imageRef2, "Same spec should produce same image tag")
	t.Logf("✅ Content-addressed caching works: %s", imageRef1)
}
