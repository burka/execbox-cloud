package k8s

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/burka/execbox/pkg/execbox"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Annotation keys
	AnnotationOriginalSpec = "execbox.io/original-spec"

	// Label keys
	LabelSessionID  = "execbox.io/session-id"
	LabelManagedBy  = "execbox.io/managed-by"
	LabelManagedVal = "execbox"
)

// SpecToPod converts execbox.Spec to Kubernetes Pod spec.
func SpecToPod(spec execbox.Spec, sessionID, namespace string, labels map[string]string) *corev1.Pod {
	// Encode original spec as JSON for recovery
	specJSON, _ := json.Marshal(spec)

	// Build pod labels
	podLabels := make(map[string]string)
	for k, v := range labels {
		podLabels[k] = v
	}
	for k, v := range spec.Labels {
		podLabels[k] = v
	}
	podLabels[LabelSessionID] = sessionID
	podLabels[LabelManagedBy] = LabelManagedVal

	// Build pod annotations
	annotations := map[string]string{
		AnnotationOriginalSpec: string(specJSON),
	}

	// Build main container
	container := corev1.Container{
		Name:       "main",
		Image:      spec.Image,
		WorkingDir: spec.WorkDir,
		Env:        EnvMapToEnvVars(spec.Env),
		TTY:        spec.TTY,
		Stdin:      true,
		StdinOnce:  true,
		Resources:  ResourcesToResourceRequirements(spec.Resources),
		Ports:      PortsToContainerPorts(spec.Ports),
	}

	// Set command if provided
	if len(spec.Command) > 0 {
		container.Command = spec.Command
	}

	// Handle BuildFiles by mounting a ConfigMap
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	if len(spec.BuildFiles) > 0 {
		configMapName := fmt.Sprintf("execbox-files-%s", sessionID[:8])
		volumes = append(volumes, corev1.Volume{
			Name: "build-files",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		})

		// Mount each file at its specified path
		for _, bf := range spec.BuildFiles {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "build-files",
				MountPath: bf.Path,
				SubPath:   strings.TrimPrefix(bf.Path, "/"),
			})
		}
	}

	// Handle structured volumes
	if len(spec.StructuredVolumes) > 0 {
		pvcPrefix := fmt.Sprintf("execbox-vol-%s", sessionID[:8])
		vols, mounts := VolumesToPodVolumes(spec.StructuredVolumes, pvcPrefix, namespace)
		volumes = append(volumes, vols...)
		volumeMounts = append(volumeMounts, mounts...)
	}

	container.VolumeMounts = volumeMounts

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("execbox-%s", sessionID[:8]),
			Namespace:   namespace,
			Labels:      podLabels,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{container},
			Volumes:       volumes,
		},
	}

	return pod
}

// EnvMapToEnvVars converts a map of environment variables to Kubernetes EnvVar slice.
func EnvMapToEnvVars(env map[string]string) []corev1.EnvVar {
	if len(env) == 0 {
		return nil
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	envVars := make([]corev1.EnvVar, 0, len(env))
	for _, k := range keys {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: env[k],
		})
	}
	return envVars
}

// ResourcesToResourceRequirements converts execbox.Resources to Kubernetes ResourceRequirements.
func ResourcesToResourceRequirements(r *execbox.Resources) corev1.ResourceRequirements {
	if r == nil {
		return corev1.ResourceRequirements{}
	}

	requirements := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	// Map CPU: prefer CPUPower, fallback to CPUMillis
	var cpuQuantity resource.Quantity
	if r.CPUPower > 0 {
		// CPUPower 1.0 = 1000m (1 core)
		cpuMillis := int64(r.CPUPower * 1000)
		cpuQuantity = *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI)
	} else if r.CPUMillis > 0 {
		cpuQuantity = *resource.NewMilliQuantity(int64(r.CPUMillis), resource.DecimalSI)
	}

	if !cpuQuantity.IsZero() {
		requirements.Requests[corev1.ResourceCPU] = cpuQuantity
		requirements.Limits[corev1.ResourceCPU] = cpuQuantity
	}

	// Map Memory
	if r.MemoryMB > 0 {
		memQuantity := *resource.NewQuantity(int64(r.MemoryMB)*1024*1024, resource.BinarySI)
		requirements.Requests[corev1.ResourceMemory] = memQuantity
		requirements.Limits[corev1.ResourceMemory] = memQuantity
	}

	return requirements
}

// PortsToContainerPorts converts execbox.Port slice to Kubernetes ContainerPort slice.
func PortsToContainerPorts(ports []execbox.Port) []corev1.ContainerPort {
	if len(ports) == 0 {
		return nil
	}

	containerPorts := make([]corev1.ContainerPort, 0, len(ports))
	for _, p := range ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: int32(p.Container),
			Protocol:      ProtocolToK8s(p.Protocol),
		})
	}
	return containerPorts
}

// ProtocolToK8s converts protocol string to Kubernetes Protocol.
func ProtocolToK8s(protocol string) corev1.Protocol {
	switch strings.ToLower(protocol) {
	case "udp":
		return corev1.ProtocolUDP
	case "sctp":
		return corev1.ProtocolSCTP
	default:
		return corev1.ProtocolTCP
	}
}

// PodToSessionInfo converts Kubernetes Pod to execbox.SessionInfo.
func PodToSessionInfo(pod *corev1.Pod, sessionID string) execbox.SessionInfo {
	info := execbox.SessionInfo{
		ID:        sessionID,
		Status:    PodPhaseToStatus(pod.Status.Phase, pod.Status.ContainerStatuses),
		CreatedAt: pod.CreationTimestamp.Time,
	}

	// Extract container ID if available
	if len(pod.Status.ContainerStatuses) > 0 {
		info.ContainerID = pod.Status.ContainerStatuses[0].ContainerID

		// Extract exit code if terminated
		if term := pod.Status.ContainerStatuses[0].State.Terminated; term != nil {
			exitCode := int(term.ExitCode)
			info.ExitCode = &exitCode
			if term.Reason != "" || term.Message != "" {
				info.Error = fmt.Sprintf("%s: %s", term.Reason, term.Message)
			}
		}
	}

	// Try to recover original spec from annotations
	if specJSON, ok := pod.Annotations[AnnotationOriginalSpec]; ok {
		var spec execbox.Spec
		if err := json.Unmarshal([]byte(specJSON), &spec); err == nil {
			info.Spec = spec
		} else {
			// Fallback: reconstruct spec from pod
			info.Spec = PodToSpec(pod)
		}
	} else {
		info.Spec = PodToSpec(pod)
	}

	// Build network info
	info.Network = BuildNetworkInfo(pod)

	return info
}

// PodToSpec reconstructs execbox.Spec from Pod (best effort).
func PodToSpec(pod *corev1.Pod) execbox.Spec {
	if len(pod.Spec.Containers) == 0 {
		return execbox.Spec{}
	}

	container := pod.Spec.Containers[0]

	spec := execbox.Spec{
		Image:     container.Image,
		Command:   container.Command,
		WorkDir:   container.WorkingDir,
		Env:       EnvVarsToMap(container.Env),
		TTY:       container.TTY,
		Labels:    extractUserLabels(pod.Labels),
		Resources: ResourceRequirementsToResources(container.Resources),
		Ports:     ContainerPortsToPorts(container.Ports),
	}

	return spec
}

// EnvVarsToMap converts Kubernetes EnvVar slice to map.
func EnvVarsToMap(envVars []corev1.EnvVar) map[string]string {
	if len(envVars) == 0 {
		return nil
	}

	env := make(map[string]string, len(envVars))
	for _, ev := range envVars {
		// Only include simple value vars (not ValueFrom)
		if ev.Value != "" {
			env[ev.Name] = ev.Value
		}
	}
	return env
}

// extractUserLabels removes execbox system labels.
func extractUserLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	userLabels := make(map[string]string)
	for k, v := range labels {
		if !strings.HasPrefix(k, "execbox.io/") {
			userLabels[k] = v
		}
	}

	if len(userLabels) == 0 {
		return nil
	}
	return userLabels
}

// ResourceRequirementsToResources converts Kubernetes ResourceRequirements to execbox.Resources.
func ResourceRequirementsToResources(rr corev1.ResourceRequirements) *execbox.Resources {
	r := &execbox.Resources{}

	// Extract CPU from limits (prefer limits over requests)
	if cpu, ok := rr.Limits[corev1.ResourceCPU]; ok {
		millis := cpu.MilliValue()
		r.CPUMillis = int(millis)
		r.CPUPower = float32(millis) / 1000.0
	} else if cpu, ok := rr.Requests[corev1.ResourceCPU]; ok {
		millis := cpu.MilliValue()
		r.CPUMillis = int(millis)
		r.CPUPower = float32(millis) / 1000.0
	}

	// Extract Memory from limits
	if mem, ok := rr.Limits[corev1.ResourceMemory]; ok {
		bytes := mem.Value()
		r.MemoryMB = int(bytes / (1024 * 1024))
	} else if mem, ok := rr.Requests[corev1.ResourceMemory]; ok {
		bytes := mem.Value()
		r.MemoryMB = int(bytes / (1024 * 1024))
	}

	// Return nil if empty
	if r.CPUMillis == 0 && r.MemoryMB == 0 {
		return nil
	}

	return r
}

// ContainerPortsToPorts converts Kubernetes ContainerPort slice to execbox.Port slice.
func ContainerPortsToPorts(containerPorts []corev1.ContainerPort) []execbox.Port {
	if len(containerPorts) == 0 {
		return nil
	}

	ports := make([]execbox.Port, 0, len(containerPorts))
	for _, cp := range containerPorts {
		ports = append(ports, execbox.Port{
			Container: int(cp.ContainerPort),
			Protocol:  K8sToProtocol(cp.Protocol),
		})
	}
	return ports
}

// K8sToProtocol converts Kubernetes Protocol to protocol string.
func K8sToProtocol(protocol corev1.Protocol) string {
	switch protocol {
	case corev1.ProtocolUDP:
		return "udp"
	case corev1.ProtocolSCTP:
		return "sctp"
	default:
		return "tcp"
	}
}

// PodPhaseToStatus maps Kubernetes pod phase to execbox.Status.
func PodPhaseToStatus(phase corev1.PodPhase, containerStatuses []corev1.ContainerStatus) execbox.Status {
	switch phase {
	case corev1.PodPending:
		return execbox.StatusPending

	case corev1.PodRunning:
		// Check if container is actually running
		if len(containerStatuses) > 0 {
			cs := containerStatuses[0]
			if cs.State.Running != nil {
				return execbox.StatusRunning
			}
			if cs.State.Waiting != nil {
				return execbox.StatusPending
			}
		}
		return execbox.StatusRunning

	case corev1.PodSucceeded:
		return execbox.StatusStopped

	case corev1.PodFailed:
		return execbox.StatusFailed

	case corev1.PodUnknown:
		return execbox.StatusPending

	default:
		return execbox.StatusPending
	}
}

// BuildNetworkInfo constructs NetworkInfo from Pod.
func BuildNetworkInfo(pod *corev1.Pod) *execbox.NetworkInfo {
	if len(pod.Spec.Containers) == 0 || len(pod.Spec.Containers[0].Ports) == 0 {
		return nil
	}

	info := &execbox.NetworkInfo{
		Mode:      string(execbox.NetworkExposed),
		Host:      "localhost", // Port forward access
		IPAddress: pod.Status.PodIP,
		Ports:     make(map[int]execbox.PortInfo),
	}

	// Map container ports (actual host ports will be set by port forwarder)
	for _, cp := range pod.Spec.Containers[0].Ports {
		containerPort := int(cp.ContainerPort)
		info.Ports[containerPort] = execbox.PortInfo{
			HostPort: containerPort, // Will be updated by port forwarder
			Protocol: K8sToProtocol(cp.Protocol),
		}
	}

	return info
}

// BuildFilesToConfigMap converts BuildFiles to a Kubernetes ConfigMap.
func BuildFilesToConfigMap(files []execbox.BuildFile, sessionID, namespace string) *corev1.ConfigMap {
	data := make(map[string]string)
	binaryData := make(map[string][]byte)

	for _, f := range files {
		key := strings.TrimPrefix(f.Path, "/")

		// Check if content is binary (contains null bytes)
		isBinary := false
		for _, b := range f.Content {
			if b == 0 {
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

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("execbox-files-%s", sessionID[:8]),
			Namespace: namespace,
			Labels: map[string]string{
				LabelSessionID: sessionID,
				LabelManagedBy: LabelManagedVal,
			},
		},
		Data:       data,
		BinaryData: binaryData,
	}

	return cm
}

// VolumesToPodVolumes converts execbox.Volume slice to Kubernetes Volume and VolumeMount slices.
func VolumesToPodVolumes(volumes []execbox.Volume, pvcPrefix, namespace string) ([]corev1.Volume, []corev1.VolumeMount) {
	if len(volumes) == 0 {
		return nil, nil
	}

	podVolumes := make([]corev1.Volume, 0, len(volumes))
	volumeMounts := make([]corev1.VolumeMount, 0, len(volumes))

	for i, v := range volumes {
		volumeName := fmt.Sprintf("vol-%d", i)
		pvcName := fmt.Sprintf("%s-%s", pvcPrefix, v.Name)

		podVolumes = append(podVolumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: v.Path,
		})
	}

	return podVolumes, volumeMounts
}
