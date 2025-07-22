package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

func setupTestServer(t *testing.T) (*Server, storage.Repository) {
	tempDir := t.TempDir()
	
	db, err := storage.NewDatabase(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	
	t.Cleanup(func() {
		db.Close()
	})
	
	repo := storage.NewSQLRepository(db)
	server := NewServer(repo, 8080)
	
	return server, repo
}

func TestHealthEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)
	
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
}

func TestCreatePod(t *testing.T) {
	server, _ := setupTestServer(t)
	
	pod := types.Pod{
		Metadata: types.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
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
	
	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("Failed to marshal pod: %v", err)
	}
	
	req, err := http.NewRequest("POST", "/api/v1/pods", bytes.NewBuffer(podJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var createdPod types.Pod
	if err := json.Unmarshal(rr.Body.Bytes(), &createdPod); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if createdPod.Metadata.Name != pod.Metadata.Name {
		t.Errorf("Expected name %s, got %s", pod.Metadata.Name, createdPod.Metadata.Name)
	}
	
	if createdPod.Metadata.UID == "" {
		t.Error("Expected UID to be set")
	}
	
	if createdPod.Status.Phase != "Pending" {
		t.Errorf("Expected phase 'Pending', got %s", createdPod.Status.Phase)
	}
}

func TestCreatePodValidationError(t *testing.T) {
	server, _ := setupTestServer(t)
	
	// Pod without containers (invalid)
	pod := types.Pod{
		Metadata: types.ObjectMeta{
			Name:      "invalid-pod",
			Namespace: "default",
		},
		Spec: types.PodSpec{
			Containers: []types.Container{}, // Empty containers
		},
	}
	
	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("Failed to marshal pod: %v", err)
	}
	
	req, err := http.NewRequest("POST", "/api/v1/pods", bytes.NewBuffer(podJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, status)
	}
	
	var errorResponse ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}
	
	if errorResponse.Error != "VALIDATION_ERROR" {
		t.Errorf("Expected error 'VALIDATION_ERROR', got %s", errorResponse.Error)
	}
}

func TestGetPod(t *testing.T) {
	server, repo := setupTestServer(t)
	
	// Create a pod first
	pod := types.Pod{
		Metadata: types.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
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
	
	// Create via repository
	specJSON, _ := json.Marshal(pod.Spec)
	statusJSON, _ := json.Marshal(types.PodStatus{Phase: "Running"})
	
	resource := storage.Resource{
		ID:        "test-pod-123",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test-pod",
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}
	
	if err := repo.CreateResource(resource); err != nil {
		t.Fatalf("Failed to create test pod: %v", err)
	}
	
	// Test GET request
	req, err := http.NewRequest("GET", "/api/v1/pods/test-pod", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var retrievedPod types.Pod
	if err := json.Unmarshal(rr.Body.Bytes(), &retrievedPod); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if retrievedPod.Metadata.Name != "test-pod" {
		t.Errorf("Expected name 'test-pod', got %s", retrievedPod.Metadata.Name)
	}
	
	if retrievedPod.Status.Phase != "Running" {
		t.Errorf("Expected phase 'Running', got %s", retrievedPod.Status.Phase)
	}
}

func TestGetPodNotFound(t *testing.T) {
	server, _ := setupTestServer(t)
	
	req, err := http.NewRequest("GET", "/api/v1/pods/non-existent", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, status)
	}
	
	var errorResponse ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}
	
	if errorResponse.Error != "RESOURCE_NOT_FOUND" {
		t.Errorf("Expected error 'RESOURCE_NOT_FOUND', got %s", errorResponse.Error)
	}
}

func TestListPods(t *testing.T) {
	server, repo := setupTestServer(t)
	
	// Create test pods
	pods := []string{"pod1", "pod2", "pod3"}
	for _, podName := range pods {
		specJSON, _ := json.Marshal(types.PodSpec{
			Containers: []types.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		})
		
		resource := storage.Resource{
			ID:        podName + "-123",
			Kind:      "Pod",
			Namespace: "default",
			Name:      podName,
			Spec:      string(specJSON),
			Status:    `{"phase":"Running"}`,
		}
		
		if err := repo.CreateResource(resource); err != nil {
			t.Fatalf("Failed to create test pod %s: %v", podName, err)
		}
	}
	
	// Test LIST request
	req, err := http.NewRequest("GET", "/api/v1/pods", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	items, ok := response["items"].([]interface{})
	if !ok {
		t.Fatal("Expected 'items' field in response")
	}
	
	if len(items) != 3 {
		t.Errorf("Expected 3 pods, got %d", len(items))
	}
	
	if response["kind"] != "PodList" {
		t.Errorf("Expected kind 'PodList', got %v", response["kind"])
	}
}

func TestDeletePod(t *testing.T) {
	server, repo := setupTestServer(t)
	
	// Create a pod first
	specJSON, _ := json.Marshal(types.PodSpec{
		Containers: []types.Container{
			{Name: "nginx", Image: "nginx:latest"},
		},
	})
	
	resource := storage.Resource{
		ID:        "test-pod-123",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test-pod",
		Spec:      string(specJSON),
		Status:    `{"phase":"Running"}`,
	}
	
	if err := repo.CreateResource(resource); err != nil {
		t.Fatalf("Failed to create test pod: %v", err)
	}
	
	// Test DELETE request
	req, err := http.NewRequest("DELETE", "/api/v1/pods/test-pod", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	// Verify pod is deleted
	_, err = repo.GetResource("Pod", "default", "test-pod")
	if err == nil {
		t.Error("Expected error when getting deleted pod")
	}
}

func TestCreateService(t *testing.T) {
	server, _ := setupTestServer(t)
	
	service := types.Service{
		Metadata: types.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: types.ServiceSpec{
			Selector: map[string]string{
				"app": "test",
			},
			Ports: []types.ServicePort{
				{
					Port:       80,
					TargetPort: 8080,
					Protocol:   "TCP",
				},
			},
			Type: "ClusterIP",
		},
	}
	
	serviceJSON, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("Failed to marshal service: %v", err)
	}
	
	req, err := http.NewRequest("POST", "/api/v1/services", bytes.NewBuffer(serviceJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var createdService types.Service
	if err := json.Unmarshal(rr.Body.Bytes(), &createdService); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if createdService.Metadata.Name != service.Metadata.Name {
		t.Errorf("Expected name %s, got %s", service.Metadata.Name, createdService.Metadata.Name)
	}
	
	if createdService.Metadata.UID == "" {
		t.Error("Expected UID to be set")
	}
}

func TestCreateDeployment(t *testing.T) {
	server, _ := setupTestServer(t)
	
	deployment := types.Deployment{
		Metadata: types.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: types.DeploymentSpec{
			Replicas: 3,
			Selector: types.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Template: types.PodTemplateSpec{
				Metadata: types.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: types.PodSpec{
					Containers: []types.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	
	deploymentJSON, err := json.Marshal(deployment)
	if err != nil {
		t.Fatalf("Failed to marshal deployment: %v", err)
	}
	
	req, err := http.NewRequest("POST", "/api/v1/deployments", bytes.NewBuffer(deploymentJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var createdDeployment types.Deployment
	if err := json.Unmarshal(rr.Body.Bytes(), &createdDeployment); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if createdDeployment.Metadata.Name != deployment.Metadata.Name {
		t.Errorf("Expected name %s, got %s", deployment.Metadata.Name, createdDeployment.Metadata.Name)
	}
	
	if createdDeployment.Spec.Replicas != deployment.Spec.Replicas {
		t.Errorf("Expected replicas %d, got %d", deployment.Spec.Replicas, createdDeployment.Spec.Replicas)
	}
	
	if createdDeployment.Status.UnavailableReplicas != deployment.Spec.Replicas {
		t.Errorf("Expected unavailable replicas %d, got %d", deployment.Spec.Replicas, createdDeployment.Status.UnavailableReplicas)
	}
}

func TestCreateNode(t *testing.T) {
	server, _ := setupTestServer(t)
	
	node := types.Node{
		Metadata: types.ObjectMeta{
			Name: "test-node",
		},
		Status: types.NodeStatus{
			Capacity: types.ResourceList{
				"cpu":    "4",
				"memory": "8Gi",
			},
			Allocatable: types.ResourceList{
				"cpu":    "3.5",
				"memory": "7Gi",
			},
			Addresses: []types.NodeAddress{
				{
					Type:    "InternalIP",
					Address: "192.168.1.100",
				},
			},
		},
	}
	
	nodeJSON, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Failed to marshal node: %v", err)
	}
	
	req, err := http.NewRequest("POST", "/api/v1/nodes", bytes.NewBuffer(nodeJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var createdNode types.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &createdNode); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if createdNode.Metadata.Name != node.Metadata.Name {
		t.Errorf("Expected name %s, got %s", node.Metadata.Name, createdNode.Metadata.Name)
	}
	
	if createdNode.Status.Capacity["cpu"] != node.Status.Capacity["cpu"] {
		t.Errorf("Expected CPU capacity %s, got %s", node.Status.Capacity["cpu"], createdNode.Status.Capacity["cpu"])
	}
	
	if len(createdNode.Status.Conditions) == 0 {
		t.Error("Expected node conditions to be initialized")
	}
}

func TestGetService(t *testing.T) {
	server, repo := setupTestServer(t)
	
	// Create a service first
	service := types.Service{
		Metadata: types.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: types.ServiceSpec{
			Ports: []types.ServicePort{
				{Port: 80, TargetPort: 8080},
			},
		},
	}
	
	specJSON, _ := json.Marshal(service.Spec)
	statusJSON, _ := json.Marshal(types.ServiceStatus{})
	
	resource := storage.Resource{
		ID:        "test-service-123",
		Kind:      "Service",
		Namespace: "default",
		Name:      "test-service",
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}
	
	if err := repo.CreateResource(resource); err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}
	
	// Test GET request
	req, err := http.NewRequest("GET", "/api/v1/services/test-service", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var retrievedService types.Service
	if err := json.Unmarshal(rr.Body.Bytes(), &retrievedService); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if retrievedService.Metadata.Name != "test-service" {
		t.Errorf("Expected name 'test-service', got %s", retrievedService.Metadata.Name)
	}
}

func TestListServices(t *testing.T) {
	server, repo := setupTestServer(t)
	
	// Create test services
	services := []string{"service1", "service2"}
	for _, serviceName := range services {
		specJSON, _ := json.Marshal(types.ServiceSpec{
			Ports: []types.ServicePort{{Port: 80}},
		})
		
		resource := storage.Resource{
			ID:        serviceName + "-123",
			Kind:      "Service",
			Namespace: "default",
			Name:      serviceName,
			Spec:      string(specJSON),
			Status:    `{}`,
		}
		
		if err := repo.CreateResource(resource); err != nil {
			t.Fatalf("Failed to create test service %s: %v", serviceName, err)
		}
	}
	
	// Test LIST request
	req, err := http.NewRequest("GET", "/api/v1/services", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	items, ok := response["items"].([]interface{})
	if !ok {
		t.Fatal("Expected 'items' field in response")
	}
	
	if len(items) != 2 {
		t.Errorf("Expected 2 services, got %d", len(items))
	}
	
	if response["kind"] != "ServiceList" {
		t.Errorf("Expected kind 'ServiceList', got %v", response["kind"])
	}
}

func TestNodeHeartbeat(t *testing.T) {
	server, repo := setupTestServer(t)
	
	// Create a node first
	node := &types.Node{
		Metadata: types.ObjectMeta{
			Name: "test-node",
			UID:  "node-123",
		},
		Status: types.NodeStatus{
			Addresses: []types.NodeAddress{
				{Type: "InternalIP", Address: "192.168.1.100"},
			},
		},
	}
	
	if err := repo.CreateNode(node); err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	
	// Test heartbeat update
	req, err := http.NewRequest("POST", "/api/v1/nodes/test-node/heartbeat", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if response["message"] != "Node heartbeat updated successfully" {
		t.Errorf("Expected success message, got %v", response["message"])
	}
}