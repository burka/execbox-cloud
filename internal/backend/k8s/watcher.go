package k8s

import (
	"context"
	"fmt"

	"github.com/burka/execbox/pkg/execbox"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

				// Remove handle from backend map to prevent memory leak
				b.removeHandle(h.ID())
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
