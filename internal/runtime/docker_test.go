package runtime

import (
	"context"
	"testing"
	"time"

	"mini-k8s-orchestration/pkg/types"
)

func TestPodToContainerSpecs(t *testing.T) {
	pod := &types.Pod{
		Metadata: types.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       "pod-123",
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []types.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      "TCP",
						},
					},
					Env: []types.EnvVar{
						{
							Name:  "ENV_VAR",
							Value: "test-value",
						},
					},
					Resources: types.ResourceRequirements{
						Requests: types.ResourceList{
							"memory": "64Mi",
							"cpu":    "250m",
						},
						Limits: types.ResourceList{
							"memory": "128Mi",
							"cpu":    "500m",
						},
					},
				},
			},
			RestartPolicy: "Always",
		},
	}
	
	specs, err := PodToContainerSpecs(pod)
	if err != nil {
		t.Fatalf("Failed to convert pod to container specs: %v", err)
	}
	
	if len(specs) != 1 {
		t.Errorf("Expected 1 container spec, got %d", len(specs))
	}
	
	spec := specs[0]
	
	// Test basic fields
	if spec.Name != "nginx" {
		t.Errorf("Expected name 'nginx', got '%s'", spec.Name)
	}
	if spec.Image != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got '%s'", spec.Image)
	}
	if spec.RestartPolicy != "Always" {
		t.Errorf("Expected restart policy 'Always', got '%s'", spec.RestartPolicy)
	}
	
	// Test environment variables
	if len(spec.Env) != 1 {
		t.Errorf("Expected 1 env var, got %d", len(spec.Env))
	} else {
		if spec.Env[0].Name != "ENV_VAR" || spec.Env[0].Value != "test-value" {
			t.Errorf("Expected env var ENV_VAR=test-value, got %s=%s", spec.Env[0].Name, spec.Env[0].Value)
		}
	}
	
	// Test port mappings
	if len(spec.Ports) != 1 {
		t.Errorf("Expected 1 port mapping, got %d", len(spec.Ports))
	} else {
		if spec.Ports[0].ContainerPort != 80 || spec.Ports[0].Protocol != "TCP" {
			t.Errorf("Expected port 80/TCP, got %d/%s", spec.Ports[0].ContainerPort, spec.Ports[0].Protocol)
		}
	}
	
	// Test resource constraints
	if spec.Resources == nil {
		t.Error("Expected resource constraints to be set")
	} else {
		if spec.Resources.MemoryLimit != "128Mi" {
			t.Errorf("Expected memory limit '128Mi', got '%s'", spec.Resources.MemoryLimit)
		}
		if spec.Resources.CPULimit != "500m" {
			t.Errorf("Expected CPU limit '500m', got '%s'", spec.Resources.CPULimit)
		}
		if spec.Resources.MemoryRequest != "64Mi" {
			t.Errorf("Expected memory request '64Mi', got '%s'", spec.Resources.MemoryRequest)
		}
		if spec.Resources.CPURequest != "250m" {
			t.Errorf("Expected CPU request '250m', got '%s'", spec.Resources.CPURequest)
		}
	}
	
	// Test labels
	expectedLabels := map[string]string{
		"pod.name":       "test-pod",
		"pod.namespace":  "default",
		"pod.uid":        "pod-123",
		"container.name": "nginx",
	}
	
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := spec.Labels[key]; !ok {
			t.Errorf("Expected label '%s' to be present", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected label '%s' to be '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"", 0, false},
		{"100", 100, false},
		{"100B", 100, false},
		{"1Ki", 1024, false},
		{"1Mi", 1024 * 1024, false},
		{"1Gi", 1024 * 1024 * 1024, false},
		{"1K", 1000, false},
		{"1M", 1000 * 1000, false},
		{"1G", 1000 * 1000 * 1000, false},
		{"512Mi", 512 * 1024 * 1024, false},
		{"1.5Gi", int64(1.5 * 1024 * 1024 * 1024), false},
		{"invalid", 0, true},
		{"100X", 0, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemory(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMemory(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("parseMemory(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseCPU(t *testing.T) {
	tests := []struct {
		input         string
		expectedQuota int64
		expectedPeriod int64
		wantErr       bool
	}{
		{"", 0, 0, false},
		{"1", 100000, 100000, false},
		{"0.5", 50000, 100000, false},
		{"500m", 50000, 100000, false},
		{"1000m", 100000, 100000, false},
		{"250m", 25000, 100000, false},
		{"invalid", 0, 0, true},
		{"100x", 0, 0, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			quota, period, err := parseCPU(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCPU(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if quota != tt.expectedQuota {
				t.Errorf("parseCPU(%q) quota = %v, want %v", tt.input, quota, tt.expectedQuota)
			}
			if period != tt.expectedPeriod {
				t.Errorf("parseCPU(%q) period = %v, want %v", tt.input, period, tt.expectedPeriod)
			}
		})
	}
}

func TestParseDockerTime(t *testing.T) {
	timeStr := "2023-01-01T12:00:00.123456789Z"
	result, err := parseDockerTime(timeStr)
	if err != nil {
		t.Errorf("parseDockerTime(%q) error = %v", timeStr, err)
	}
	
	expected := time.Date(2023, 1, 1, 12, 0, 0, 123456789, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("parseDockerTime(%q) = %v, want %v", timeStr, result, expected)
	}
}

// Integration tests that require Docker daemon
// These tests will be skipped if Docker is not available

func TestDockerRuntimeIntegration(t *testing.T) {
	// Skip if Docker is not available
	runtime, err := NewDockerRuntime()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer runtime.Close()
	
	ctx := context.Background()
	
	// Test ping
	if err := runtime.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}
	
	t.Run("ListImages", func(t *testing.T) {
		images, err := runtime.ListImages(ctx)
		if err != nil {
			t.Errorf("Failed to list images: %v", err)
		}
		
		// Should return at least empty list
		if images == nil {
			t.Error("Expected non-nil images list")
		}
	})
	
	t.Run("ListContainers", func(t *testing.T) {
		containers, err := runtime.ListContainers(ctx, true)
		if err != nil {
			t.Errorf("Failed to list containers: %v", err)
		}
		
		// Should return at least empty list
		if containers == nil {
			t.Error("Expected non-nil containers list")
		}
	})
}

func TestDockerRuntimeContainerLifecycle(t *testing.T) {
	// Skip if Docker is not available
	runtime, err := NewDockerRuntime()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer runtime.Close()
	
	ctx := context.Background()
	
	// Test ping
	if err := runtime.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}
	
	// Pull a small test image
	testImage := "hello-world:latest"
	if err := runtime.PullImage(ctx, testImage); err != nil {
		t.Skipf("Failed to pull test image: %v", err)
	}
	
	// Create container spec
	spec := &ContainerSpec{
		Name:  "test-container",
		Image: testImage,
		Labels: map[string]string{
			"test": "true",
		},
		RestartPolicy: "Never",
	}
	
	// Create container
	containerID, err := runtime.CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	// Cleanup container at the end
	defer func() {
		runtime.RemoveContainer(ctx, containerID, true)
	}()
	
	// Test container status
	status, err := runtime.GetContainerStatus(ctx, containerID)
	if err != nil {
		t.Errorf("Failed to get container status: %v", err)
	} else {
		if status.ID != containerID {
			t.Errorf("Expected container ID %s, got %s", containerID, status.ID)
		}
		if status.State != "created" {
			t.Errorf("Expected container state 'created', got %s", status.State)
		}
	}
	
	// Start container
	if err := runtime.StartContainer(ctx, containerID); err != nil {
		t.Errorf("Failed to start container: %v", err)
	}
	
	// Wait a moment for container to run
	time.Sleep(2 * time.Second)
	
	// Check status again
	status, err = runtime.GetContainerStatus(ctx, containerID)
	if err != nil {
		t.Errorf("Failed to get container status after start: %v", err)
	} else {
		// hello-world container should exit quickly
		if status.State != "exited" {
			t.Logf("Container state: %s (expected 'exited', but container might still be running)", status.State)
		}
	}
	
	// Test container logs
	logs, err := runtime.GetContainerLogs(ctx, containerID, false)
	if err != nil {
		t.Errorf("Failed to get container logs: %v", err)
	} else {
		logs.Close()
	}
}