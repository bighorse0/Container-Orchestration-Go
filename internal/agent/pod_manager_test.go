package agent

import (
	"context"
	"io"
	"testing"

	"mini-k8s-orchestration/internal/runtime"
	"mini-k8s-orchestration/pkg/types"
)

// MockContainerRuntime is a mock implementation of runtime.ContainerRuntime
type MockContainerRuntime struct {
	containers map[string]*runtime.ContainerStatus
	images     map[string]bool
}

func NewMockContainerRuntime() *MockContainerRuntime {
	return &MockContainerRuntime{
		containers: make(map[string]*runtime.ContainerStatus),
		images:     make(map[string]bool),
	}
}

func (m *MockContainerRuntime) CreateContainer(ctx context.Context, spec *runtime.ContainerSpec) (string, error) {
	containerID := "container-" + spec.Name
	m.containers[containerID] = &runtime.ContainerStatus{
		ID:    containerID,
		Name:  spec.Name,
		State: "created",
		Image: spec.Image,
	}
	return containerID, nil
}

func (m *MockContainerRuntime) StartContainer(ctx context.Context, containerID string) error {
	if container, exists := m.containers[containerID]; exists {
		container.State = "running"
	}
	return nil
}

func (m *MockContainerRuntime) StopContainer(ctx context.Context, containerID string, timeout int) error {
	if container, exists := m.containers[containerID]; exists {
		container.State = "exited"
	}
	return nil
}

func (m *MockContainerRuntime) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	delete(m.containers, containerID)
	return nil
}

func (m *MockContainerRuntime) GetContainerStatus(ctx context.Context, containerID string) (*runtime.ContainerStatus, error) {
	if container, exists := m.containers[containerID]; exists {
		return container, nil
	}
	return nil, nil
}

func (m *MockContainerRuntime) ListContainers(ctx context.Context, all bool) ([]*runtime.ContainerInfo, error) {
	var containers []*runtime.ContainerInfo
	for id, container := range m.containers {
		containers = append(containers, &runtime.ContainerInfo{
			ID:     id,
			Names:  []string{container.Name},
			Image:  container.Image,
			State:  container.State,
			Status: container.State,
		})
	}
	return containers, nil
}

func (m *MockContainerRuntime) PullImage(ctx context.Context, image string) error {
	m.images[image] = true
	return nil
}

func (m *MockContainerRuntime) ListImages(ctx context.Context) ([]*runtime.ImageInfo, error) {
	var images []*runtime.ImageInfo
	for image := range m.images {
		images = append(images, &runtime.ImageInfo{
			RepoTags: []string{image},
		})
	}
	return images, nil
}

func (m *MockContainerRuntime) GetContainerLogs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockContainerRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	return nil
}

func (m *MockContainerRuntime) Ping(ctx context.Context) error {
	return nil
}

func TestPodManager(t *testing.T) {
	// Create mock container runtime
	mockRuntime := NewMockContainerRuntime()
	
	// Create pod manager
	podManager := NewPodManager(mockRuntime)
	
	// Create test pod
	pod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
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
				},
			},
		},
	}
	
	// Test creating a pod
	err := podManager.SyncPods([]*types.Pod{pod})
	if err != nil {
		t.Fatalf("Failed to sync pods: %v", err)
	}
	
	// Verify container was created
	containers, err := mockRuntime.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}
	
	if len(containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(containers))
	}
	
	if containers[0].Image != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got '%s'", containers[0].Image)
	}
	
	// Test deleting a pod
	err = podManager.SyncPods([]*types.Pod{})
	if err != nil {
		t.Fatalf("Failed to sync pods: %v", err)
	}
	
	// Verify container was deleted
	containers, err = mockRuntime.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}
	
	if len(containers) != 0 {
		t.Fatalf("Expected 0 containers, got %d", len(containers))
	}
}