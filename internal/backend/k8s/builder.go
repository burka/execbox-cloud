package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/burka/execbox/pkg/execbox"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// KanikoImage is the Kaniko executor image for building containers.
	KanikoImage = "gcr.io/kaniko-project/executor:latest"

	// DefaultImageTTL is the default TTL for built images on ttl.sh.
	DefaultImageTTL = "4h"

	// BuildTimeout is the maximum time allowed for a build.
	BuildTimeout = 10 * time.Minute

	// ImageHashLength is the length of the hash used for image tags.
	ImageHashLength = 16
)

// Builder builds container images using Kaniko inside K8s.
type Builder struct {
	clientset kubernetes.Interface
	namespace string
	registry  string // Default: ttl.sh
	imageTTL  string // TTL for images on ttl.sh
}

// BuilderConfig configures the Builder.
type BuilderConfig struct {
	Registry string // Image registry (default: ttl.sh)
	ImageTTL string // Image TTL for ttl.sh (default: 4h)
}

// NewBuilder creates a new K8s image builder.
func NewBuilder(clientset kubernetes.Interface, namespace string, cfg BuilderConfig) *Builder {
	registry := cfg.Registry
	if registry == "" {
		registry = "ttl.sh"
	}

	imageTTL := cfg.ImageTTL
	if imageTTL == "" {
		imageTTL = DefaultImageTTL
	}

	return &Builder{
		clientset: clientset,
		namespace: namespace,
		registry:  registry,
		imageTTL:  imageTTL,
	}
}

// BuildSpec defines what to build.
type BuildSpec struct {
	BaseImage string             // Base image (FROM)
	Setup     []string           // Setup commands (RUN)
	Files     []execbox.BuildFile // Files to include (COPY)
}

// Build builds an image and returns the tag.
// Uses content-addressed hashing for caching - same inputs = same tag.
func (b *Builder) Build(ctx context.Context, spec BuildSpec) (string, error) {
	// Generate Dockerfile
	dockerfile := b.generateDockerfile(spec)

	// Compute content-addressed tag
	tag := b.computeImageTag(dockerfile, spec.Files)

	// Full image reference
	imageRef := fmt.Sprintf("%s/%s:%s", b.registry, tag, b.imageTTL)

	// Create build resources
	buildID := uuid.New().String()[:8]
	configMapName := fmt.Sprintf("kaniko-build-%s", buildID)
	podName := fmt.Sprintf("kaniko-build-%s", buildID)

	// Create ConfigMap with Dockerfile and files
	cm := b.createBuildConfigMap(configMapName, dockerfile, spec.Files)
	if _, err := b.clientset.CoreV1().ConfigMaps(b.namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		return "", fmt.Errorf("failed to create build configmap: %w", err)
	}

	// Cleanup on exit
	defer func() {
		_ = b.clientset.CoreV1().ConfigMaps(b.namespace).Delete(context.Background(), configMapName, metav1.DeleteOptions{})
		_ = b.clientset.CoreV1().Pods(b.namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	}()

	// Create Kaniko pod
	pod := b.createKanikoPod(podName, configMapName, imageRef)
	if _, err := b.clientset.CoreV1().Pods(b.namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return "", fmt.Errorf("failed to create kaniko pod: %w", err)
	}

	// Wait for build to complete
	if err := b.waitForBuild(ctx, podName); err != nil {
		// Get pod logs for debugging
		logs := b.getPodLogs(ctx, podName)
		return "", fmt.Errorf("build failed: %w\nLogs:\n%s", err, logs)
	}

	return imageRef, nil
}

// generateDockerfile creates a Dockerfile from the build spec.
func (b *Builder) generateDockerfile(spec BuildSpec) string {
	var lines []string

	// FROM instruction
	lines = append(lines, fmt.Sprintf("FROM %s", spec.BaseImage))

	// COPY files (sorted for deterministic output)
	if len(spec.Files) > 0 {
		sortedFiles := make([]execbox.BuildFile, len(spec.Files))
		copy(sortedFiles, spec.Files)
		sort.Slice(sortedFiles, func(i, j int) bool {
			return sortedFiles[i].Path < sortedFiles[j].Path
		})

		for _, f := range sortedFiles {
			// Files are stored in ConfigMap, mounted at /build
			srcName := sanitizePathForConfigMapKey(f.Path)
			lines = append(lines, fmt.Sprintf("COPY %s %s", srcName, f.Path))
		}
	}

	// RUN instructions (setup commands)
	for _, cmd := range spec.Setup {
		// Validate: prevent FROM injection
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(cmd)), "FROM") {
			continue // Skip malicious FROM attempts
		}
		lines = append(lines, fmt.Sprintf("RUN %s", cmd))
	}

	return strings.Join(lines, "\n")
}

// computeImageTag generates a content-addressed tag for the image.
func (b *Builder) computeImageTag(dockerfile string, files []execbox.BuildFile) string {
	h := sha256.New()

	// Hash Dockerfile content
	h.Write([]byte(dockerfile))

	// Hash file contents (sorted for determinism)
	if len(files) > 0 {
		sortedFiles := make([]execbox.BuildFile, len(files))
		copy(sortedFiles, files)
		sort.Slice(sortedFiles, func(i, j int) bool {
			return sortedFiles[i].Path < sortedFiles[j].Path
		})

		for _, f := range sortedFiles {
			h.Write([]byte(f.Path))
			h.Write(f.Content)
		}
	}

	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("execbox-%s", hash[:ImageHashLength])
}

// createBuildConfigMap creates a ConfigMap containing the Dockerfile and build files.
func (b *Builder) createBuildConfigMap(name, dockerfile string, files []execbox.BuildFile) *corev1.ConfigMap {
	data := map[string]string{
		"Dockerfile": dockerfile,
	}
	binaryData := map[string][]byte{}

	for _, f := range files {
		key := sanitizePathForConfigMapKey(f.Path)

		// Check if content is binary
		isBinary := false
		for _, byte := range f.Content {
			if byte == 0 {
				isBinary = true
				break
			}
		}

		if isBinary {
			binaryData[key] = f.Content
		} else {
			data[key] = string(f.Content)
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: b.namespace,
			Labels: map[string]string{
				LabelManagedBy: LabelManagedVal,
			},
		},
		Data:       data,
		BinaryData: binaryData,
	}
}

// createKanikoPod creates a Kaniko pod for building the image.
// Uses an init container to copy ConfigMap files (resolving symlinks) to an emptyDir
// that Kaniko then uses as the build context.
func (b *Builder) createKanikoPod(name, configMapName, imageRef string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: b.namespace,
			Labels: map[string]string{
				LabelManagedBy: LabelManagedVal,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			// Init container copies ConfigMap files to emptyDir, resolving symlinks
			InitContainers: []corev1.Container{
				{
					Name:  "copy-context",
					Image: "alpine:latest",
					Command: []string{
						"sh", "-c",
						// cp -L follows symlinks, copying actual file contents
						"cp -rL /configmap/* /build/",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "configmap-source",
							MountPath: "/configmap",
						},
						{
							Name:      "build-context",
							MountPath: "/build",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "kaniko",
					Image: KanikoImage,
					Args: []string{
						"--dockerfile=/build/Dockerfile",
						"--context=dir:///build",
						fmt.Sprintf("--destination=%s", imageRef),
						"--cache=false", // Could enable with cache-repo for faster builds
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "build-context",
							MountPath: "/build",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("1"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("512Mi"),
							corev1.ResourceCPU:    resource.MustParse("250m"),
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					// ConfigMap with Dockerfile and build files (mounted as symlinks)
					Name: "configmap-source",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
						},
					},
				},
				{
					// emptyDir for actual build context (real files, no symlinks)
					Name: "build-context",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}

// waitForBuild waits for the Kaniko pod to complete.
func (b *Builder) waitForBuild(ctx context.Context, podName string) error {
	ctx, cancel := context.WithTimeout(ctx, BuildTimeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("build timeout: %w", ctx.Err())

		case <-ticker.C:
			pod, err := b.clientset.CoreV1().Pods(b.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get build pod: %w", err)
			}

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				return nil

			case corev1.PodFailed:
				reason := "unknown"
				if len(pod.Status.ContainerStatuses) > 0 {
					if term := pod.Status.ContainerStatuses[0].State.Terminated; term != nil {
						reason = fmt.Sprintf("%s: %s", term.Reason, term.Message)
					}
				}
				return fmt.Errorf("build failed: %s", reason)

			case corev1.PodPending, corev1.PodRunning:
				// Still building, continue waiting
			}
		}
	}
}

// getPodLogs retrieves logs from a pod for debugging.
func (b *Builder) getPodLogs(ctx context.Context, podName string) string {
	req := b.clientset.CoreV1().Pods(b.namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: int64Ptr(100),
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Sprintf("(failed to get logs: %v)", err)
	}
	defer stream.Close()

	buf := make([]byte, 8192)
	n, _ := stream.Read(buf)
	return string(buf[:n])
}

func int64Ptr(i int64) *int64 {
	return &i
}
