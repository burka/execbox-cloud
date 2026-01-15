//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/backend/k8s"
	"github.com/burka/execbox/pkg/execbox"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestBuilderDebug(t *testing.T) {
	config, _ := clientcmd.BuildConfigFromFlags("", "/tmp/microk8s-kubeconfig")
	clientset, _ := kubernetes.NewForConfig(config)

	builder := k8s.NewBuilder(clientset, "execbox-test", k8s.BuilderConfig{
		Registry: "ttl.sh",
		ImageTTL: "1h",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	spec := k8s.BuildSpec{
		BaseImage: "alpine:latest",
		Files: []execbox.BuildFile{
			{
				Path:    "/app/config.json",
				Content: []byte(`{"name": "test", "version": "v2-fixed"}`),
			},
		},
	}

	imageRef, err := builder.Build(ctx, spec)
	if err != nil {
		t.Logf("Build error (with logs): %v", err)
		t.FailNow()
	}
	t.Logf("Built: %s", imageRef)

	// Verify the file exists in the image
	backend, err := k8s.NewBackend(k8s.BackendConfig{
		Kubeconfig: "/tmp/microk8s-kubeconfig",
		Namespace:  "execbox-test",
	})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Run a container with the built image and cat the file
	handle, err := backend.Run(ctx, execbox.Spec{
		Image:   imageRef,
		Command: []string{"cat", "/app/config.json"},
	})
	if err != nil {
		t.Fatalf("Failed to run container: %v", err)
	}
	defer backend.Destroy(ctx, handle.ID())

	result := <-handle.Wait()
	t.Logf("Exit code: %d", result.Code)

	stdout := handle.Stdout()
	buf := make([]byte, 1024)
	n, _ := stdout.Read(buf)
	output := string(buf[:n])
	t.Logf("Output: %q", output)

	// Also list what's in /app
	handle2, err := backend.Run(ctx, execbox.Spec{
		Image:   imageRef,
		Command: []string{"ls", "-la", "/app"},
	})
	if err != nil {
		t.Logf("Failed to list /app: %v", err)
	} else {
		defer backend.Destroy(ctx, handle2.ID())
		<-handle2.Wait()
		stdout2 := handle2.Stdout()
		buf2 := make([]byte, 1024)
		n2, _ := stdout2.Read(buf2)
		t.Logf("/app contents: %q", string(buf2[:n2]))
	}

	// List root to see what's there
	handle3, err := backend.Run(ctx, execbox.Spec{
		Image:   imageRef,
		Command: []string{"ls", "-la", "/"},
	})
	if err != nil {
		t.Logf("Failed to list /: %v", err)
	} else {
		defer backend.Destroy(ctx, handle3.ID())
		<-handle3.Wait()
		stdout3 := handle3.Stdout()
		buf3 := make([]byte, 2048)
		n3, _ := stdout3.Read(buf3)
		t.Logf("/ contents: %q", string(buf3[:n3]))
	}
}
