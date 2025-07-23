package controller

import (
	"encoding/json"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// Helper functions for creating test resources
func createTestPod(name, namespace string, labels map[string]string, ready bool, podIP string) storage.Resource {
	metadata := types.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		UID:       "pod-" + name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	spec := types.PodSpec{
		Containers: []types.Container{
			{
				Name:  "test-container",
				Image: "test:latest",
				Ports: []types.ContainerPort{
					{
						Name:          "http",
						ContainerPort: 8080,
						Protocol:      "TCP",
					},
				},
			},
		},
	}

	status := types.PodStatus{
		Phase: "Running",
		PodIP: podIP,
		ContainerStatuses: []types.ContainerStatus{
			{
				Name:  "test-container",
				Ready: ready,
			},
		},
	}

	if ready {
		status.Conditions = []types.PodCondition{
			{
				Type:   "Ready",
				Status: "True",
			},
		}
	}

	metadataJSON, _ := json.Marshal(metadata)
	specJSON, _ := json.Marshal(spec)
	statusJSON, _ := json.Marshal(status)

	return storage.Resource{
		ID:        metadata.UID,
		Kind:      "Pod",
		Namespace: namespace,
		Name:      name,
		Metadata:  string(metadataJSON),
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: metadata.CreatedAt,
		UpdatedAt: metadata.UpdatedAt,
	}
}

func createTestService(name, namespace string, selector map[string]string, ports []types.ServicePort) storage.Resource {
	metadata := types.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		UID:       "service-" + name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	spec := types.ServiceSpec{
		Selector: selector,
		Ports:    ports,
	}

	status := types.ServiceStatus{
		Endpoints: []types.Endpoint{},
	}

	metadataJSON, _ := json.Marshal(metadata)
	specJSON, _ := json.Marshal(spec)
	statusJSON, _ := json.Marshal(status)

	return storage.Resource{
		ID:        metadata.UID,
		Kind:      "Service",
		Namespace: namespace,
		Name:      name,
		Metadata:  string(metadataJSON),
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: metadata.CreatedAt,
		UpdatedAt: metadata.UpdatedAt,
	}
}

func TestServiceController_matchesSelector(t *testing.T) {
	sc := NewServiceController(NewMockRepository())

	tests := []struct {
		name        string
		podLabels   map[string]string
		selector    map[string]string
		shouldMatch bool
	}{
		{
			name:        "exact match",
			podLabels:   map[string]string{"app": "web", "version": "v1"},
			selector:    map[string]string{"app": "web"},
			shouldMatch: true,
		},
		{
			name:        "multiple labels match",
			podLabels:   map[string]string{"app": "web", "version": "v1"},
			selector:    map[string]string{"app": "web", "version": "v1"},
			shouldMatch: true,
		},
		{
			name:        "no match",
			podLabels:   map[string]string{"app": "web", "version": "v1"},
			selector:    map[string]string{"app": "api"},
			shouldMatch: false,
		},
		{
			name:        "partial match",
			podLabels:   map[string]string{"app": "web"},
			selector:    map[string]string{"app": "web", "version": "v1"},
			shouldMatch: false,
		},
		{
			name:        "empty selector",
			podLabels:   map[string]string{"app": "web"},
			selector:    map[string]string{},
			shouldMatch: false,
		},
		{
			name:        "nil pod labels",
			podLabels:   nil,
			selector:    map[string]string{"app": "web"},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sc.matchesSelector(tt.podLabels, tt.selector)
			if result != tt.shouldMatch {
				t.Errorf("matchesSelector() = %v, want %v", result, tt.shouldMatch)
			}
		})
	}
}

func TestServiceController_isPodReady(t *testing.T) {
	sc := NewServiceController(NewMockRepository())

	tests := []struct {
		name      string
		pod       *types.Pod
		shouldBeReady bool
	}{
		{
			name: "running pod with ready condition",
			pod: &types.Pod{
				Status: types.PodStatus{
					Phase: "Running",
					PodIP: "10.0.0.1",
					Conditions: []types.PodCondition{
						{Type: "Ready", Status: "True"},
					},
				},
			},
			shouldBeReady: true,
		},
		{
			name: "running pod with ready containers",
			pod: &types.Pod{
				Status: types.PodStatus{
					Phase: "Running",
					PodIP: "10.0.0.1",
					ContainerStatuses: []types.ContainerStatus{
						{Ready: true},
					},
				},
			},
			shouldBeReady: true,
		},
		{
			name: "pending pod",
			pod: &types.Pod{
				Status: types.PodStatus{
					Phase: "Pending",
					PodIP: "10.0.0.1",
				},
			},
			shouldBeReady: false,
		},
		{
			name: "pod without IP",
			pod: &types.Pod{
				Status: types.PodStatus{
					Phase: "Running",
					PodIP: "",
				},
			},
			shouldBeReady: false,
		},
		{
			name: "pod with not ready condition",
			pod: &types.Pod{
				Status: types.PodStatus{
					Phase: "Running",
					PodIP: "10.0.0.1",
					Conditions: []types.PodCondition{
						{Type: "Ready", Status: "False"},
					},
				},
			},
			shouldBeReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sc.isPodReady(tt.pod)
			if result != tt.shouldBeReady {
				t.Errorf("isPodReady() = %v, want %v", result, tt.shouldBeReady)
			}
		})
	}
}

func TestServiceController_findMatchingPods(t *testing.T) {
	repo := NewMockRepository()
	sc := NewServiceController(repo)

	// Create test pods
	pod1 := createTestPod("pod1", "default", map[string]string{"app": "web"}, true, "10.0.0.1")
	pod2 := createTestPod("pod2", "default", map[string]string{"app": "web", "version": "v1"}, true, "10.0.0.2")
	pod3 := createTestPod("pod3", "default", map[string]string{"app": "api"}, true, "10.0.0.3")
	pod4 := createTestPod("pod4", "other", map[string]string{"app": "web"}, true, "10.0.0.4") // different namespace
	pod5 := createTestPod("pod5", "default", map[string]string{"app": "web"}, false, "10.0.0.5") // not ready

	pods := []storage.Resource{pod1, pod2, pod3, pod4, pod5}

	tests := []struct {
		name           string
		selector       map[string]string
		namespace      string
		expectedCount  int
		expectedNames  []string
	}{
		{
			name:          "match single label",
			selector:      map[string]string{"app": "web"},
			namespace:     "default",
			expectedCount: 2,
			expectedNames: []string{"pod1", "pod2"},
		},
		{
			name:          "match multiple labels",
			selector:      map[string]string{"app": "web", "version": "v1"},
			namespace:     "default",
			expectedCount: 1,
			expectedNames: []string{"pod2"},
		},
		{
			name:          "no matches",
			selector:      map[string]string{"app": "database"},
			namespace:     "default",
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "different namespace",
			selector:      map[string]string{"app": "web"},
			namespace:     "other",
			expectedCount: 1,
			expectedNames: []string{"pod4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := sc.findMatchingPods(tt.selector, pods, tt.namespace)
			if err != nil {
				t.Fatalf("findMatchingPods() error = %v", err)
			}

			if len(matches) != tt.expectedCount {
				t.Errorf("findMatchingPods() count = %v, want %v", len(matches), tt.expectedCount)
			}

			// Check that the right pods were matched
			matchedNames := make([]string, len(matches))
			for i, match := range matches {
				matchedNames[i] = match.Name
			}

			for _, expectedName := range tt.expectedNames {
				found := false
				for _, matchedName := range matchedNames {
					if matchedName == expectedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected pod %s not found in matches", expectedName)
				}
			}
		})
	}
}

func TestServiceController_buildEndpoints(t *testing.T) {
	sc := NewServiceController(NewMockRepository())

	// Create test pods
	pod1 := createTestPod("pod1", "default", map[string]string{"app": "web"}, true, "10.0.0.1")
	pod2 := createTestPod("pod2", "default", map[string]string{"app": "web"}, true, "10.0.0.2")

	pods := []storage.Resource{pod1, pod2}

	servicePorts := []types.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: 8080,
			Protocol:   "TCP",
		},
	}

	endpoints, err := sc.buildEndpoints(pods, servicePorts)
	if err != nil {
		t.Fatalf("buildEndpoints() error = %v", err)
	}

	expectedCount := 2 // 2 pods * 1 service port
	if len(endpoints) != expectedCount {
		t.Errorf("buildEndpoints() count = %v, want %v", len(endpoints), expectedCount)
	}

	// Check endpoint details
	expectedIPs := []string{"10.0.0.1", "10.0.0.2"}
	for _, endpoint := range endpoints {
		if endpoint.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", endpoint.Port)
		}
		if !endpoint.Ready {
			t.Errorf("Expected endpoint to be ready")
		}

		found := false
		for _, expectedIP := range expectedIPs {
			if endpoint.IP == expectedIP {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected endpoint IP: %s", endpoint.IP)
		}
	}
}

func TestServiceController_reconcileService(t *testing.T) {
	repo := NewMockRepository()
	sc := NewServiceController(repo)

	// Create test service
	servicePorts := []types.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: 8080,
			Protocol:   "TCP",
		},
	}
	service := createTestService("test-service", "default", map[string]string{"app": "web"}, servicePorts)
	repo.CreateResource(service)

	// Create test pods
	pod1 := createTestPod("pod1", "default", map[string]string{"app": "web"}, true, "10.0.0.1")
	pod2 := createTestPod("pod2", "default", map[string]string{"app": "web"}, true, "10.0.0.2")
	pod3 := createTestPod("pod3", "default", map[string]string{"app": "api"}, true, "10.0.0.3") // different label

	pods := []storage.Resource{pod1, pod2, pod3}

	// Reconcile the service
	err := sc.reconcileService(service, pods)
	if err != nil {
		t.Fatalf("reconcileService() error = %v", err)
	}

	// Check that the service was updated with endpoints
	updatedService, err := repo.GetResource("Service", "default", "test-service")
	if err != nil {
		t.Fatalf("Failed to get updated service: %v", err)
	}

	var status types.ServiceStatus
	err = json.Unmarshal([]byte(updatedService.Status), &status)
	if err != nil {
		t.Fatalf("Failed to unmarshal service status: %v", err)
	}

	expectedEndpointCount := 2 // 2 matching pods * 1 service port
	if len(status.Endpoints) != expectedEndpointCount {
		t.Errorf("Expected %d endpoints, got %d", expectedEndpointCount, len(status.Endpoints))
	}

	// Verify endpoint details
	for _, endpoint := range status.Endpoints {
		if endpoint.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", endpoint.Port)
		}
		if !endpoint.Ready {
			t.Errorf("Expected endpoint to be ready")
		}
		if endpoint.IP != "10.0.0.1" && endpoint.IP != "10.0.0.2" {
			t.Errorf("Unexpected endpoint IP: %s", endpoint.IP)
		}
	}
}

func TestServiceController_endpointsEqual(t *testing.T) {
	sc := NewServiceController(NewMockRepository())

	endpoint1 := types.Endpoint{IP: "10.0.0.1", Port: 8080, Ready: true}
	endpoint2 := types.Endpoint{IP: "10.0.0.2", Port: 8080, Ready: true}
	endpoint3 := types.Endpoint{IP: "10.0.0.1", Port: 8080, Ready: false}

	tests := []struct {
		name     string
		a        []types.Endpoint
		b        []types.Endpoint
		expected bool
	}{
		{
			name:     "identical endpoints",
			a:        []types.Endpoint{endpoint1, endpoint2},
			b:        []types.Endpoint{endpoint1, endpoint2},
			expected: true,
		},
		{
			name:     "different order",
			a:        []types.Endpoint{endpoint1, endpoint2},
			b:        []types.Endpoint{endpoint2, endpoint1},
			expected: true,
		},
		{
			name:     "different readiness",
			a:        []types.Endpoint{endpoint1},
			b:        []types.Endpoint{endpoint3},
			expected: false,
		},
		{
			name:     "different count",
			a:        []types.Endpoint{endpoint1},
			b:        []types.Endpoint{endpoint1, endpoint2},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []types.Endpoint{},
			b:        []types.Endpoint{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sc.endpointsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("endpointsEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}