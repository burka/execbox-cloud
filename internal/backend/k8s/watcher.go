package k8s

import (
	"context"
	"fmt"

	"github.com/burka/execbox/pkg/execbox"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// watchPod watches for pod phase changes and signals exit when the pod terminates.
func (b *Backend) watchPod(ctx context.Context, h *Handle, podName string) {
	watcher, err := b.clientset.CoreV1().Pods(b.config.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
	})
	if err != nil {
		// If we can't create the watcher, signal an error
		h.SignalExit(execbox.ExitResult{
			Code:  -1,
			Error: fmt.Errorf("failed to create pod watcher: %w", err),
		})
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context canceled, stop watching
			return

		case event, ok := <-watcher.ResultChan():
			if !ok {
				// Channel closed, stop watching
				return
			}

			// Handle pod deletion (from Kill or Destroy)
			// Note: We remove the handle here because the pod was explicitly deleted,
			// which means no one will need to read the output anymore.
			if event.Type == watch.Deleted {
				h.SignalExit(execbox.ExitResult{
					Code:  137, // SIGKILL exit code
					Error: nil,
				})
				b.removeHandle(h.ID())
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Check if pod has terminated
			switch pod.Status.Phase {
			case corev1.PodSucceeded, corev1.PodFailed:
				// Pod has terminated, extract exit code
				exitCode := 0
				var exitErr error

				if pod.Status.Phase == corev1.PodFailed {
					exitErr = fmt.Errorf("pod failed")
				}

				// Try to get exit code from container status
				if len(pod.Status.ContainerStatuses) > 0 {
					containerStatus := pod.Status.ContainerStatuses[0]
					if containerStatus.State.Terminated != nil {
						exitCode = int(containerStatus.State.Terminated.ExitCode)
						if exitCode != 0 {
							exitErr = fmt.Errorf("container exited with code %d", exitCode)
						} else {
							exitErr = nil // Successful exit
						}
					}
				}

				// Signal exit
				h.SignalExit(execbox.ExitResult{
					Code:  exitCode,
					Error: exitErr,
				})

				// Note: Do NOT remove the handle here. Clients may still want to
				// attach and read the buffered output after the pod completes.
				// The handle will be removed when:
				// - The pod is explicitly deleted (Kill/Destroy)
				// - The backend is closed
				return
			}
		}
	}
}

// startWatching starts the pod watcher for a handle.
func (b *Backend) startWatching(h *Handle, podName string) {
	watchCtx, watchCancel := context.WithCancel(context.Background())
	h.SetCancelWatcher(watchCancel)
	go b.watchPod(watchCtx, h, podName)
}
