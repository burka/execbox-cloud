package k8s

import (
	"strings"
	"testing"

	"github.com/burka/execbox/pkg/execbox"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSanitizePathForConfigMapKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path with leading slash",
			input:    "/app/config.txt",
			expected: "app-config.txt",
		},
		{
			name:     "path without leading slash",
			input:    "app/config.txt",
			expected: "app-config.txt",
		},
		{
			name:     "deeply nested path",
			input:    "/var/lib/app/config/database.yml",
			expected: "var-lib-app-config-database.yml",
		},
		{
			name:     "root level file",
			input:    "/config.txt",
			expected: "config.txt",
		},
		{
			name:     "file without directory",
			input:    "config.txt",
			expected: "config.txt",
		},
		{
			name:     "path with multiple slashes",
			input:    "/app//config.txt",
			expected: "app--config.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePathForConfigMapKey(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizePathForConfigMapKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			// Verify the result doesn't contain "/"
			if strings.Contains(result, "/") {
				t.Errorf("sanitized key %q still contains slash", result)
			}
		})
	}
}

func TestEnvMapToEnvVars(t *testing.T) {
	env := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	envVars := EnvMapToEnvVars(env)

	if len(envVars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(envVars))
	}

	// Check sorted order
	if envVars[0].Name != "KEY1" || envVars[0].Value != "value1" {
		t.Errorf("unexpected first env var: %+v", envVars[0])
	}
	if envVars[1].Name != "KEY2" || envVars[1].Value != "value2" {
		t.Errorf("unexpected second env var: %+v", envVars[1])
	}
}

func TestEnvMapToEnvVarsEmpty(t *testing.T) {
	envVars := EnvMapToEnvVars(nil)
	if envVars != nil {
		t.Errorf("expected nil for empty env, got %v", envVars)
	}
}

func TestProtocolToK8s(t *testing.T) {
	tests := []struct {
		input    string
		expected corev1.Protocol
	}{
		{"tcp", corev1.ProtocolTCP},
		{"TCP", corev1.ProtocolTCP},
		{"udp", corev1.ProtocolUDP},
		{"UDP", corev1.ProtocolUDP},
		{"sctp", corev1.ProtocolSCTP},
		{"", corev1.ProtocolTCP},
		{"unknown", corev1.ProtocolTCP},
	}

	for _, tt := range tests {
		result := ProtocolToK8s(tt.input)
		if result != tt.expected {
			t.Errorf("ProtocolToK8s(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestK8sToProtocol(t *testing.T) {
	tests := []struct {
		input    corev1.Protocol
		expected string
	}{
		{corev1.ProtocolTCP, "tcp"},
		{corev1.ProtocolUDP, "udp"},
		{corev1.ProtocolSCTP, "sctp"},
	}

	for _, tt := range tests {
		result := K8sToProtocol(tt.input)
		if result != tt.expected {
			t.Errorf("K8sToProtocol(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestPortsToContainerPorts(t *testing.T) {
	ports := []execbox.Port{
		{Container: 8080, Protocol: "tcp"},
		{Container: 9090, Protocol: "udp"},
	}

	containerPorts := PortsToContainerPorts(ports)

	if len(containerPorts) != 2 {
		t.Fatalf("expected 2 container ports, got %d", len(containerPorts))
	}

	if containerPorts[0].ContainerPort != 8080 || containerPorts[0].Protocol != corev1.ProtocolTCP {
		t.Errorf("unexpected first port: %+v", containerPorts[0])
	}
	if containerPorts[1].ContainerPort != 9090 || containerPorts[1].Protocol != corev1.ProtocolUDP {
		t.Errorf("unexpected second port: %+v", containerPorts[1])
	}
}

func TestResourcesToResourceRequirements(t *testing.T) {
	tests := []struct {
		name     string
		input    *execbox.Resources
		checkCPU bool
		cpuValue int64
		checkMem bool
		memValue int64
	}{
		{
			name:     "nil resources",
			input:    nil,
			checkCPU: false,
			checkMem: false,
		},
		{
			name: "CPUPower",
			input: &execbox.Resources{
				CPUPower: 1.5,
				MemoryMB: 512,
			},
			checkCPU: true,
			cpuValue: 1500,
			checkMem: true,
			memValue: 512 * 1024 * 1024,
		},
		{
			name: "CPUMillis",
			input: &execbox.Resources{
				CPUMillis: 2000,
				MemoryMB:  1024,
			},
			checkCPU: true,
			cpuValue: 2000,
			checkMem: true,
			memValue: 1024 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := ResourcesToResourceRequirements(tt.input)

			if tt.checkCPU {
				cpu := rr.Limits[corev1.ResourceCPU]
				if cpu.MilliValue() != tt.cpuValue {
					t.Errorf("CPU = %dm, want %dm", cpu.MilliValue(), tt.cpuValue)
				}

				// Check that requests == limits
				cpuReq := rr.Requests[corev1.ResourceCPU]
				if cpuReq.MilliValue() != tt.cpuValue {
					t.Errorf("CPU request = %dm, want %dm", cpuReq.MilliValue(), tt.cpuValue)
				}
			}

			if tt.checkMem {
				mem := rr.Limits[corev1.ResourceMemory]
				if mem.Value() != tt.memValue {
					t.Errorf("Memory = %d, want %d", mem.Value(), tt.memValue)
				}

				// Check that requests == limits
				memReq := rr.Requests[corev1.ResourceMemory]
				if memReq.Value() != tt.memValue {
					t.Errorf("Memory request = %d, want %d", memReq.Value(), tt.memValue)
				}
			}
		})
	}
}

func TestResourceRequirementsToResources(t *testing.T) {
	tests := []struct {
		name      string
		input     corev1.ResourceRequirements
		expectNil bool
		cpuPower  float32
		cpuMillis int
		memoryMB  int
	}{
		{
			name: "with limits",
			input: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(1500, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(512*1024*1024, resource.BinarySI),
				},
			},
			cpuPower:  1.5,
			cpuMillis: 1500,
			memoryMB:  512,
		},
		{
			name: "with requests only",
			input: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(1024*1024*1024, resource.BinarySI),
				},
			},
			cpuPower:  2.0,
			cpuMillis: 2000,
			memoryMB:  1024,
		},
		{
			name:      "empty",
			input:     corev1.ResourceRequirements{},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ResourceRequirementsToResources(tt.input)

			if tt.expectNil {
				if r != nil {
					t.Errorf("expected nil, got %+v", r)
				}
				return
			}

			if r.CPUPower != tt.cpuPower {
				t.Errorf("CPUPower = %f, want %f", r.CPUPower, tt.cpuPower)
			}
			if r.CPUMillis != tt.cpuMillis {
				t.Errorf("CPUMillis = %d, want %d", r.CPUMillis, tt.cpuMillis)
			}
			if r.MemoryMB != tt.memoryMB {
				t.Errorf("MemoryMB = %d, want %d", r.MemoryMB, tt.memoryMB)
			}
		})
	}
}

func TestPodPhaseToStatus(t *testing.T) {
	tests := []struct {
		name            string
		phase           corev1.PodPhase
		containerStatus []corev1.ContainerStatus
		expectedStatus  execbox.Status
	}{
		{
			name:           "pending",
			phase:          corev1.PodPending,
			expectedStatus: execbox.StatusPending,
		},
		{
			name:  "running with container running",
			phase: corev1.PodRunning,
			containerStatus: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
			expectedStatus: execbox.StatusRunning,
		},
		{
			name:  "running with container waiting",
			phase: corev1.PodRunning,
			containerStatus: []corev1.ContainerStatus{
				{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}},
			},
			expectedStatus: execbox.StatusPending,
		},
		{
			name:           "succeeded",
			phase:          corev1.PodSucceeded,
			expectedStatus: execbox.StatusStopped,
		},
		{
			name:           "failed",
			phase:          corev1.PodFailed,
			expectedStatus: execbox.StatusFailed,
		},
		{
			name:           "unknown",
			phase:          corev1.PodUnknown,
			expectedStatus: execbox.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := PodPhaseToStatus(tt.phase, tt.containerStatus)
			if status != tt.expectedStatus {
				t.Errorf("status = %v, want %v", status, tt.expectedStatus)
			}
		})
	}
}

func TestSpecToPodBasic(t *testing.T) {
	spec := execbox.Spec{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo hello"},
		Env: map[string]string{
			"FOO": "bar",
		},
		WorkDir: "/workspace",
		TTY:     true,
		Resources: &execbox.Resources{
			CPUPower: 1.0,
			MemoryMB: 512,
		},
		Ports: []execbox.Port{
			{Container: 8080, Protocol: "tcp"},
		},
	}

	sessionID := "test-session-12345678"
	namespace := "default"
	labels := map[string]string{
		"env": "test",
	}

	pod := SpecToPod(spec, sessionID, namespace, labels)

	if pod.Name != "execbox-test-ses" {
		t.Errorf("pod name = %q, want %q", pod.Name, "execbox-test-ses")
	}
	if pod.Namespace != namespace {
		t.Errorf("namespace = %q, want %q", pod.Namespace, namespace)
	}

	// Check labels
	if pod.Labels[LabelSessionID] != sessionID {
		t.Errorf("session-id label = %q, want %q", pod.Labels[LabelSessionID], sessionID)
	}
	if pod.Labels[LabelManagedBy] != LabelManagedVal {
		t.Errorf("managed-by label = %q, want %q", pod.Labels[LabelManagedBy], LabelManagedVal)
	}
	if pod.Labels["env"] != "test" {
		t.Errorf("custom label = %q, want %q", pod.Labels["env"], "test")
	}

	// Check container
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
	}

	container := pod.Spec.Containers[0]
	if container.Name != "main" {
		t.Errorf("container name = %q, want %q", container.Name, "main")
	}
	if container.Image != "alpine:latest" {
		t.Errorf("image = %q, want %q", container.Image, "alpine:latest")
	}
	if !container.TTY {
		t.Error("TTY should be true")
	}
	if !container.Stdin {
		t.Error("Stdin should be true")
	}

	// Check restart policy
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("restart policy = %v, want %v", pod.Spec.RestartPolicy, corev1.RestartPolicyNever)
	}
}

func TestBuildFilesToConfigMap(t *testing.T) {
	files := []execbox.BuildFile{
		{Path: "/app/config.txt", Content: []byte("text content")},
		{Path: "/app/binary", Content: []byte{0x00, 0x01, 0x02}}, // Binary with null byte
	}

	sessionID := "test-session-12345678"
	namespace := "default"

	cm := BuildFilesToConfigMap(files, sessionID, namespace)

	if cm.Name != "execbox-files-test-ses" {
		t.Errorf("configmap name = %q, want %q", cm.Name, "execbox-files-test-ses")
	}
	if cm.Namespace != namespace {
		t.Errorf("namespace = %q, want %q", cm.Namespace, namespace)
	}

	// Check text file in Data (path "/" replaced with "-")
	if text, ok := cm.Data["app-config.txt"]; !ok {
		t.Error("expected app-config.txt in Data")
	} else if text != "text content" {
		t.Errorf("text content = %q, want %q", text, "text content")
	}

	// Check binary file in BinaryData (path "/" replaced with "-")
	if binary, ok := cm.BinaryData["app-binary"]; !ok {
		t.Error("expected app-binary in BinaryData")
	} else if len(binary) != 3 || binary[0] != 0x00 {
		t.Errorf("binary content = %v, want [0 1 2]", binary)
	}
}

func TestVolumesToPodVolumes(t *testing.T) {
	volumes := []execbox.Volume{
		{Name: "cache", Path: "/cache"},
		{Name: "data", Path: "/data"},
	}

	pvcPrefix := "execbox-vol-abc123"
	namespace := "default"

	podVolumes, volumeMounts := VolumesToPodVolumes(volumes, pvcPrefix, namespace)

	if len(podVolumes) != 2 {
		t.Fatalf("expected 2 pod volumes, got %d", len(podVolumes))
	}
	if len(volumeMounts) != 2 {
		t.Fatalf("expected 2 volume mounts, got %d", len(volumeMounts))
	}

	// Check first volume
	if podVolumes[0].Name != "vol-0" {
		t.Errorf("volume name = %q, want %q", podVolumes[0].Name, "vol-0")
	}
	if podVolumes[0].PersistentVolumeClaim.ClaimName != "execbox-vol-abc123-cache" {
		t.Errorf("pvc name = %q, want %q", podVolumes[0].PersistentVolumeClaim.ClaimName, "execbox-vol-abc123-cache")
	}

	if volumeMounts[0].Name != "vol-0" {
		t.Errorf("mount name = %q, want %q", volumeMounts[0].Name, "vol-0")
	}
	if volumeMounts[0].MountPath != "/cache" {
		t.Errorf("mount path = %q, want %q", volumeMounts[0].MountPath, "/cache")
	}
}

func TestPodToSessionInfo(t *testing.T) {
	tests := []struct {
		name              string
		pod               *corev1.Pod
		sessionID         string
		expectedStatus    execbox.Status
		expectedExitCode  *int
		expectedError     string
		expectedContID    string
		expectSpecInAnnot bool
	}{
		{
			name: "running pod with container status",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "execbox-abc123",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: metav1.Now().Time},
					Annotations: map[string]string{
						AnnotationOriginalSpec: `{"image":"alpine:latest","command":["sh"]}`,
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							ContainerID: "docker://abc123def456",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			sessionID:         "abc123",
			expectedStatus:    execbox.StatusRunning,
			expectedContID:    "docker://abc123def456",
			expectSpecInAnnot: true,
		},
		{
			name: "failed pod with exit code",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "execbox-def456",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: metav1.Now().Time},
					Annotations: map[string]string{
						AnnotationOriginalSpec: `{"image":"python:3.12"}`,
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							ContainerID: "docker://def456ghi789",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 127,
									Reason:   "Error",
									Message:  "command not found",
								},
							},
						},
					},
				},
			},
			sessionID:         "def456",
			expectedStatus:    execbox.StatusFailed,
			expectedExitCode:  intPtr(127),
			expectedError:     "Error: command not found",
			expectedContID:    "docker://def456ghi789",
			expectSpecInAnnot: true,
		},
		{
			name: "pod without container statuses",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "execbox-ghi789",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: metav1.Now().Time},
					Annotations: map[string]string{
						AnnotationOriginalSpec: `{"image":"ubuntu:22.04"}`,
					},
				},
				Status: corev1.PodStatus{
					Phase:             corev1.PodPending,
					ContainerStatuses: []corev1.ContainerStatus{},
				},
			},
			sessionID:         "ghi789",
			expectedStatus:    execbox.StatusPending,
			expectSpecInAnnot: true,
		},
		{
			name: "pod without original spec annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "execbox-jkl012",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: metav1.Now().Time},
					Annotations:       map[string]string{},
					Labels: map[string]string{
						"app":          "test",
						LabelSessionID: "jkl012",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:       "main",
							Image:      "nginx:latest",
							WorkingDir: "/app",
							Env: []corev1.EnvVar{
								{Name: "ENV_VAR", Value: "value"},
							},
							TTY: true,
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							ContainerID: "docker://jkl012mno345",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			sessionID:         "jkl012",
			expectedStatus:    execbox.StatusRunning,
			expectedContID:    "docker://jkl012mno345",
			expectSpecInAnnot: false,
		},
		{
			name: "succeeded pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "execbox-mno345",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: metav1.Now().Time},
					Annotations: map[string]string{
						AnnotationOriginalSpec: `{"image":"busybox"}`,
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							ContainerID: "docker://mno345pqr678",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 0,
								},
							},
						},
					},
				},
			},
			sessionID:         "mno345",
			expectedStatus:    execbox.StatusStopped,
			expectedExitCode:  intPtr(0),
			expectedContID:    "docker://mno345pqr678",
			expectSpecInAnnot: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := PodToSessionInfo(tt.pod, tt.sessionID)

			if info.ID != tt.sessionID {
				t.Errorf("ID = %q, want %q", info.ID, tt.sessionID)
			}

			if info.Status != tt.expectedStatus {
				t.Errorf("Status = %v, want %v", info.Status, tt.expectedStatus)
			}

			if info.ContainerID != tt.expectedContID {
				t.Errorf("ContainerID = %q, want %q", info.ContainerID, tt.expectedContID)
			}

			if tt.expectedExitCode != nil {
				if info.ExitCode == nil {
					t.Error("ExitCode is nil, expected non-nil")
				} else if *info.ExitCode != *tt.expectedExitCode {
					t.Errorf("ExitCode = %d, want %d", *info.ExitCode, *tt.expectedExitCode)
				}
			} else {
				if info.ExitCode != nil {
					t.Errorf("ExitCode = %d, want nil", *info.ExitCode)
				}
			}

			if tt.expectedError != "" {
				if info.Error != tt.expectedError {
					t.Errorf("Error = %q, want %q", info.Error, tt.expectedError)
				}
			}

			// Verify spec is populated
			if info.Spec.Image == "" {
				t.Error("Spec.Image is empty")
			}

			// For pods without annotation, check reconstruction worked
			if !tt.expectSpecInAnnot {
				if info.Spec.Image != "nginx:latest" {
					t.Errorf("Reconstructed Spec.Image = %q, want %q", info.Spec.Image, "nginx:latest")
				}
				if info.Spec.WorkDir != "/app" {
					t.Errorf("Reconstructed Spec.WorkDir = %q, want %q", info.Spec.WorkDir, "/app")
				}
			}
		})
	}
}

func TestPodToSpec(t *testing.T) {
	tests := []struct {
		name          string
		pod           *corev1.Pod
		expectedImage string
		expectedCmd   []string
		expectedEnv   map[string]string
		expectEmpty   bool
	}{
		{
			name: "reconstruction from pod spec",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "execbox-abc123",
					Namespace: "default",
					Labels: map[string]string{
						"app":              "myapp",
						"env":              "prod",
						LabelSessionID:     "abc123",
						LabelManagedBy:     LabelManagedVal,
						"execbox.io/other": "filtered",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:       "main",
							Image:      "python:3.12",
							Command:    []string{"python", "app.py"},
							WorkingDir: "/workspace",
							Env: []corev1.EnvVar{
								{Name: "PYTHONPATH", Value: "/app/lib"},
								{Name: "DEBUG", Value: "true"},
							},
							TTY: true,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(1024*1024*1024, resource.BinarySI),
								},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			expectedImage: "python:3.12",
			expectedCmd:   []string{"python", "app.py"},
			expectedEnv: map[string]string{
				"PYTHONPATH": "/app/lib",
				"DEBUG":      "true",
			},
		},
		{
			name: "empty containers",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "execbox-def456",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
			expectEmpty: true,
		},
		{
			name: "minimal pod spec",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "execbox-ghi789",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "alpine",
						},
					},
				},
			},
			expectedImage: "alpine",
			expectedCmd:   nil,
			expectedEnv:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := PodToSpec(tt.pod)

			if tt.expectEmpty {
				if spec.Image != "" {
					t.Errorf("expected empty spec, got Image = %q", spec.Image)
				}
				return
			}

			if spec.Image != tt.expectedImage {
				t.Errorf("Image = %q, want %q", spec.Image, tt.expectedImage)
			}

			if len(spec.Command) != len(tt.expectedCmd) {
				t.Errorf("Command length = %d, want %d", len(spec.Command), len(tt.expectedCmd))
			} else {
				for i, cmd := range tt.expectedCmd {
					if spec.Command[i] != cmd {
						t.Errorf("Command[%d] = %q, want %q", i, spec.Command[i], cmd)
					}
				}
			}

			if len(spec.Env) != len(tt.expectedEnv) {
				t.Errorf("Env length = %d, want %d", len(spec.Env), len(tt.expectedEnv))
			} else {
				for k, v := range tt.expectedEnv {
					if spec.Env[k] != v {
						t.Errorf("Env[%q] = %q, want %q", k, spec.Env[k], v)
					}
				}
			}

			// Verify user labels are extracted (system labels filtered)
			if tt.name == "reconstruction from pod spec" {
				if _, hasSystem := spec.Labels[LabelSessionID]; hasSystem {
					t.Errorf("Labels should not contain system label %s", LabelSessionID)
				}
				if _, hasSystem := spec.Labels[LabelManagedBy]; hasSystem {
					t.Errorf("Labels should not contain system label %s", LabelManagedBy)
				}
				if _, hasSystem := spec.Labels["execbox.io/other"]; hasSystem {
					t.Error("Labels should not contain execbox.io/ prefixed labels")
				}
				if spec.Labels["app"] != "myapp" {
					t.Errorf("Labels[app] = %q, want %q", spec.Labels["app"], "myapp")
				}
				if spec.Labels["env"] != "prod" {
					t.Errorf("Labels[env] = %q, want %q", spec.Labels["env"], "prod")
				}
			}
		})
	}
}

func TestEnvVarsToMap(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []corev1.EnvVar
		expected map[string]string
	}{
		{
			name: "normal conversion",
			envVars: []corev1.EnvVar{
				{Name: "KEY1", Value: "value1"},
				{Name: "KEY2", Value: "value2"},
				{Name: "PATH", Value: "/usr/bin:/bin"},
			},
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"PATH": "/usr/bin:/bin",
			},
		},
		{
			name:     "empty input",
			envVars:  []corev1.EnvVar{},
			expected: nil,
		},
		{
			name:     "nil input",
			envVars:  nil,
			expected: nil,
		},
		{
			name: "skip ValueFrom entries",
			envVars: []corev1.EnvVar{
				{Name: "NORMAL", Value: "value"},
				{
					Name: "FROM_SECRET",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
							Key:                  "key",
						},
					},
				},
				{Name: "ANOTHER", Value: "another"},
			},
			expected: map[string]string{
				"NORMAL":  "value",
				"ANOTHER": "another",
			},
		},
		{
			name: "empty value",
			envVars: []corev1.EnvVar{
				{Name: "EMPTY", Value: ""},
				{Name: "NOT_EMPTY", Value: "has value"},
			},
			expected: map[string]string{
				"NOT_EMPTY": "has value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnvVarsToMap(tt.envVars)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("length = %d, want %d", len(result), len(tt.expected))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("result[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestExtractUserLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected map[string]string
	}{
		{
			name: "filter execbox.io/ prefixed labels",
			labels: map[string]string{
				"app":                 "myapp",
				"env":                 "production",
				LabelSessionID:        "abc123",
				LabelManagedBy:        LabelManagedVal,
				"execbox.io/version":  "1.0",
				"execbox.io/internal": "data",
				"team":                "platform",
			},
			expected: map[string]string{
				"app":  "myapp",
				"env":  "production",
				"team": "platform",
			},
		},
		{
			name:     "empty input",
			labels:   map[string]string{},
			expected: nil,
		},
		{
			name:     "nil input",
			labels:   nil,
			expected: nil,
		},
		{
			name: "all system labels",
			labels: map[string]string{
				LabelSessionID:       "abc123",
				LabelManagedBy:       LabelManagedVal,
				"execbox.io/version": "1.0",
			},
			expected: nil,
		},
		{
			name: "no system labels",
			labels: map[string]string{
				"app":     "frontend",
				"version": "2.0",
			},
			expected: map[string]string{
				"app":     "frontend",
				"version": "2.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractUserLabels(tt.labels)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("length = %d, want %d", len(result), len(tt.expected))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("result[%q] = %q, want %q", k, result[k], v)
				}
			}

			// Verify no execbox.io/ labels are present
			for k := range result {
				if strings.HasPrefix(k, "execbox.io/") {
					t.Errorf("found system label in result: %q", k)
				}
			}
		})
	}
}

func TestContainerPortsToPorts(t *testing.T) {
	tests := []struct {
		name           string
		containerPorts []corev1.ContainerPort
		expected       []execbox.Port
	}{
		{
			name: "normal conversion",
			containerPorts: []corev1.ContainerPort{
				{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
				{ContainerPort: 9090, Protocol: corev1.ProtocolUDP},
				{ContainerPort: 3000, Protocol: corev1.ProtocolSCTP},
			},
			expected: []execbox.Port{
				{Container: 8080, Protocol: "tcp"},
				{Container: 9090, Protocol: "udp"},
				{Container: 3000, Protocol: "sctp"},
			},
		},
		{
			name:           "empty input",
			containerPorts: []corev1.ContainerPort{},
			expected:       nil,
		},
		{
			name:           "nil input",
			containerPorts: nil,
			expected:       nil,
		},
		{
			name: "single port",
			containerPorts: []corev1.ContainerPort{
				{ContainerPort: 443, Protocol: corev1.ProtocolTCP},
			},
			expected: []execbox.Port{
				{Container: 443, Protocol: "tcp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainerPortsToPorts(tt.containerPorts)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("length = %d, want %d", len(result), len(tt.expected))
			}

			for i, expected := range tt.expected {
				if result[i].Container != expected.Container {
					t.Errorf("result[%d].Container = %d, want %d", i, result[i].Container, expected.Container)
				}
				if result[i].Protocol != expected.Protocol {
					t.Errorf("result[%d].Protocol = %q, want %q", i, result[i].Protocol, expected.Protocol)
				}
			}
		})
	}
}

func TestBuildNetworkInfo(t *testing.T) {
	tests := []struct {
		name         string
		pod          *corev1.Pod
		expectNil    bool
		expectedMode string
		expectedHost string
		numPorts     int
	}{
		{
			name: "pod with ports",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "execbox-abc123",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 9090, Protocol: corev1.ProtocolUDP},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.1.100",
				},
			},
			expectedMode: string(execbox.NetworkExposed),
			expectedHost: "localhost",
			numPorts:     2,
		},
		{
			name: "pod without ports",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "execbox-def456",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "alpine:latest",
							Ports: []corev1.ContainerPort{},
						},
					},
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.1.101",
				},
			},
			expectNil: true,
		},
		{
			name: "empty containers",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "execbox-ghi789",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
			expectNil: true,
		},
		{
			name: "pod with single port",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "execbox-jkl012",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "redis:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 6379, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					PodIP: "10.0.1.102",
				},
			},
			expectedMode: string(execbox.NetworkExposed),
			expectedHost: "localhost",
			numPorts:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildNetworkInfo(tt.pod)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("result is nil, expected non-nil")
			}

			if result.Mode != tt.expectedMode {
				t.Errorf("Mode = %q, want %q", result.Mode, tt.expectedMode)
			}

			if result.Host != tt.expectedHost {
				t.Errorf("Host = %q, want %q", result.Host, tt.expectedHost)
			}

			if result.IPAddress != tt.pod.Status.PodIP {
				t.Errorf("IPAddress = %q, want %q", result.IPAddress, tt.pod.Status.PodIP)
			}

			if len(result.Ports) != tt.numPorts {
				t.Errorf("Ports length = %d, want %d", len(result.Ports), tt.numPorts)
			}

			// Verify port mappings
			for _, cp := range tt.pod.Spec.Containers[0].Ports {
				containerPort := int(cp.ContainerPort)
				portInfo, exists := result.Ports[containerPort]
				if !exists {
					t.Errorf("missing port info for container port %d", containerPort)
					continue
				}

				if portInfo.HostPort != containerPort {
					t.Errorf("HostPort = %d, want %d", portInfo.HostPort, containerPort)
				}

				expectedProtocol := K8sToProtocol(cp.Protocol)
				if portInfo.Protocol != expectedProtocol {
					t.Errorf("Protocol = %q, want %q", portInfo.Protocol, expectedProtocol)
				}
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func TestSpecToPodWithSetup(t *testing.T) {
	spec := execbox.Spec{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "cat /tmp/setup-result.txt"},
		Setup: []string{
			"echo 'setup step 1' > /tmp/setup-result.txt",
			"echo 'setup step 2' >> /tmp/setup-result.txt",
		},
		Env: map[string]string{
			"TEST": "value",
		},
		WorkDir: "/workspace",
	}

	sessionID := "test-session-12345678"
	namespace := "default"
	labels := map[string]string{}

	pod := SpecToPod(spec, sessionID, namespace, labels)

	// Check that init containers were created
	if len(pod.Spec.InitContainers) != 2 {
		t.Fatalf("expected 2 init containers, got %d", len(pod.Spec.InitContainers))
	}

	// Check first init container
	initContainer0 := pod.Spec.InitContainers[0]
	if initContainer0.Name != "setup-0" {
		t.Errorf("init container 0 name = %q, want %q", initContainer0.Name, "setup-0")
	}
	if initContainer0.Image != spec.Image {
		t.Errorf("init container 0 image = %q, want %q", initContainer0.Image, spec.Image)
	}
	if initContainer0.WorkingDir != spec.WorkDir {
		t.Errorf("init container 0 workdir = %q, want %q", initContainer0.WorkingDir, spec.WorkDir)
	}
	expectedCmd0 := []string{"/bin/sh", "-c", "echo 'setup step 1' > /tmp/setup-result.txt"}
	if len(initContainer0.Command) != len(expectedCmd0) {
		t.Fatalf("init container 0 command length = %d, want %d", len(initContainer0.Command), len(expectedCmd0))
	}
	for i, cmd := range expectedCmd0 {
		if initContainer0.Command[i] != cmd {
			t.Errorf("init container 0 command[%d] = %q, want %q", i, initContainer0.Command[i], cmd)
		}
	}

	// Check that init container has workspace volume mounted
	if len(initContainer0.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount in init container 0, got %d", len(initContainer0.VolumeMounts))
	}
	if initContainer0.VolumeMounts[0].Name != "init-workspace" {
		t.Errorf("init container 0 volume mount name = %q, want %q", initContainer0.VolumeMounts[0].Name, "init-workspace")
	}
	if initContainer0.VolumeMounts[0].MountPath != "/tmp" {
		t.Errorf("init container 0 volume mount path = %q, want %q", initContainer0.VolumeMounts[0].MountPath, "/tmp")
	}

	// Check second init container
	initContainer1 := pod.Spec.InitContainers[1]
	if initContainer1.Name != "setup-1" {
		t.Errorf("init container 1 name = %q, want %q", initContainer1.Name, "setup-1")
	}
	expectedCmd1 := []string{"/bin/sh", "-c", "echo 'setup step 2' >> /tmp/setup-result.txt"}
	if len(initContainer1.Command) != len(expectedCmd1) {
		t.Fatalf("init container 1 command length = %d, want %d", len(initContainer1.Command), len(expectedCmd1))
	}
	for i, cmd := range expectedCmd1 {
		if initContainer1.Command[i] != cmd {
			t.Errorf("init container 1 command[%d] = %q, want %q", i, initContainer1.Command[i], cmd)
		}
	}

	// Check that main container has workspace volume mounted
	mainContainer := pod.Spec.Containers[0]
	hasWorkspaceMount := false
	for _, vm := range mainContainer.VolumeMounts {
		if vm.Name == "init-workspace" && vm.MountPath == "/tmp" {
			hasWorkspaceMount = true
			break
		}
	}
	if !hasWorkspaceMount {
		t.Error("main container missing init-workspace volume mount at /tmp")
	}

	// Check that workspace volume exists in pod spec
	hasWorkspaceVolume := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "init-workspace" {
			hasWorkspaceVolume = true
			if vol.VolumeSource.EmptyDir == nil {
				t.Error("workspace volume should be an emptyDir")
			}
			break
		}
	}
	if !hasWorkspaceVolume {
		t.Error("pod spec missing init-workspace volume")
	}

	// Check that env vars are passed to init containers
	if len(initContainer0.Env) != 1 {
		t.Fatalf("expected 1 env var in init container 0, got %d", len(initContainer0.Env))
	}
	if initContainer0.Env[0].Name != "TEST" || initContainer0.Env[0].Value != "value" {
		t.Errorf("init container 0 env var = %+v, want TEST=value", initContainer0.Env[0])
	}
}

func TestSpecToPodWithoutSetup(t *testing.T) {
	spec := execbox.Spec{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	sessionID := "test-session-12345678"
	namespace := "default"
	labels := map[string]string{}

	pod := SpecToPod(spec, sessionID, namespace, labels)

	// Check that no init containers were created
	if len(pod.Spec.InitContainers) != 0 {
		t.Errorf("expected 0 init containers, got %d", len(pod.Spec.InitContainers))
	}

	// Check that no workspace volume was added
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "init-workspace" {
			t.Error("workspace volume should not exist when there are no setup commands")
		}
	}

	// Check that main container has no workspace mount
	mainContainer := pod.Spec.Containers[0]
	for _, vm := range mainContainer.VolumeMounts {
		if vm.Name == "init-workspace" {
			t.Error("main container should not have workspace volume mount when there are no setup commands")
		}
	}
}
