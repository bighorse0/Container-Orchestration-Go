package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/controller"
	"mini-k8s-orchestration/internal/loadbalancer"
	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// TestLoadBalancerServiceIntegration tests the complete integration between
// service controller and load balancer
func TestLoadBalancerServiceIntegration(t *testing.T) {
	// Initialize database with unique name for this test
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewSQLRepository(db)

	// Initialize service controller
	serviceController := controller.NewServiceController(repo)
	serviceController.Start()
	defer serviceController.Stop()

	// Initialize load balancer
	lb := loadbalancer.NewLoadBalancer(repo)
	err = lb.Start(0) // Use port 0 to get a random available port
	if err != nil {
		t.Fatalf("Failed to start load balancer: %v", err)
	}
	defer lb.Stop()

	// Create mock backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response from backend 1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response from backend 2"))
	}))
	defer backend2.Close()

	// Extract backend addresses
	backend1URL := backend1.URL[7:] // Remove "http://"
	backend2URL := backend2.URL[7:] // Remove "http://"
	
	backend1Parts := strings.Split(backend1URL, ":")
	backend2Parts := strings.Split(backend2URL, ":")

	// Create test pods that would be backing the service
	// Use the actual backend IPs and create endpoints that point to the test servers
	pod1 := createTestPodWithPort("pod1", "default", map[string]string{"app": "web"}, true, backend1Parts[0], backend1Parts[1])
	pod2 := createTestPodWithPort("pod2", "default", map[string]string{"app": "web"}, true, backend2Parts[0], backend2Parts[1])

	// Store pods in repository
	err = repo.CreateResource(pod1)
	if err != nil {
		t.Fatalf("Failed to create pod1: %v", err)
	}

	err = repo.CreateResource(pod2)
	if err != nil {
		t.Fatalf("Failed to create pod2: %v", err)
	}

	// Create test service
	service := createTestService("web-service", "default", map[string]string{"app": "web"})
	err = repo.CreateResource(service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait for service controller to reconcile endpoints
	time.Sleep(2 * time.Second)

	// Trigger service reconciliation manually to ensure endpoints are updated
	err = serviceController.ReconcileServices()
	if err != nil {
		t.Fatalf("Failed to reconcile services: %v", err)
	}

	// Wait for load balancer to pick up the service
	time.Sleep(1 * time.Second)

	// Verify that the load balancer has the service endpoints
	endpoints := lb.GetServiceEndpoints("web-service", "default")
	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Test load balancing by making multiple requests
	responses := make(map[string]int)
	
	for i := 0; i < 10; i++ {
		// Create request to the service
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "web-service.default"
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Handle request through load balancer
		lb.HandleRequest(w, req)
		
		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
			continue
		}
		
		body, _ := io.ReadAll(w.Body)
		response := string(body)
		responses[response]++
	}

	// Verify that both backends received requests (round-robin distribution)
	if len(responses) != 2 {
		t.Errorf("Expected responses from 2 backends, got %d", len(responses))
	}

	for response, count := range responses {
		if count == 0 {
			t.Errorf("Backend with response '%s' received no requests", response)
		}
		t.Logf("Backend response '%s' received %d requests", response, count)
	}
}

// TestLoadBalancerHealthBasedRouting tests that unhealthy endpoints are excluded
func TestLoadBalancerHealthBasedRouting(t *testing.T) {
	// Initialize database with unique name for this test
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewSQLRepository(db)

	// Initialize service controller
	serviceController := controller.NewServiceController(repo)
	serviceController.Start()
	defer serviceController.Stop()

	// Initialize load balancer
	lb := loadbalancer.NewLoadBalancer(repo)
	err = lb.Start(0)
	if err != nil {
		t.Fatalf("Failed to start load balancer: %v", err)
	}
	defer lb.Stop()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response from healthy backend"))
	}))
	defer backend.Close()

	backendURL := backend.URL[7:] // Remove "http://"
	backendParts := strings.Split(backendURL, ":")

	// Create test pods - one healthy, one unhealthy
	healthyPod := createTestPod("healthy-pod", "default", map[string]string{"app": "web"}, true, backendParts[0])
	unhealthyPod := createTestPod("unhealthy-pod", "default", map[string]string{"app": "web"}, false, "10.0.0.99")

	// Store pods in repository
	err = repo.CreateResource(healthyPod)
	if err != nil {
		t.Fatalf("Failed to create healthy pod: %v", err)
	}

	err = repo.CreateResource(unhealthyPod)
	if err != nil {
		t.Fatalf("Failed to create unhealthy pod: %v", err)
	}

	// Create test service
	service := createTestService("web-service", "default", map[string]string{"app": "web"})
	err = repo.CreateResource(service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait for service controller to reconcile endpoints
	time.Sleep(2 * time.Second)

	// Trigger service reconciliation
	err = serviceController.ReconcileServices()
	if err != nil {
		t.Fatalf("Failed to reconcile services: %v", err)
	}

	// Wait for load balancer to pick up the service
	time.Sleep(1 * time.Second)

	// Verify that only healthy endpoints are available
	endpoints := lb.GetServiceEndpoints("web-service", "default")
	healthyCount := 0
	for _, endpoint := range endpoints {
		if endpoint.Ready {
			healthyCount++
		}
	}

	if healthyCount != 1 {
		t.Errorf("Expected 1 healthy endpoint, got %d", healthyCount)
	}

	// Test that requests only go to healthy backend
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "web-service.default"
		
		w := httptest.NewRecorder()
		lb.HandleRequest(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
			continue
		}
		
		body, _ := io.ReadAll(w.Body)
		response := string(body)
		
		if response != "response from healthy backend" {
			t.Errorf("Request %d: Expected response from healthy backend, got: %s", i, response)
		}
	}
}

// TestLoadBalancerServiceNameResolution tests service name resolution
func TestLoadBalancerServiceNameResolution(t *testing.T) {
	// Initialize database with unique name for this test
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewSQLRepository(db)

	// Initialize service controller
	serviceController := controller.NewServiceController(repo)
	serviceController.Start()
	defer serviceController.Stop()

	// Initialize load balancer
	lb := loadbalancer.NewLoadBalancer(repo)
	err = lb.Start(0)
	if err != nil {
		t.Fatalf("Failed to start load balancer: %v", err)
	}
	defer lb.Stop()

	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("service resolved successfully"))
	}))
	defer backend.Close()

	backendURL := backend.URL[7:]
	backendParts := strings.Split(backendURL, ":")

	// Create test pod and service
	pod := createTestPod("test-pod", "production", map[string]string{"app": "api"}, true, backendParts[0])
	service := createTestService("api-service", "production", map[string]string{"app": "api"})

	err = repo.CreateResource(pod)
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}

	err = repo.CreateResource(service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait for reconciliation
	time.Sleep(2 * time.Second)
	err = serviceController.ReconcileServices()
	if err != nil {
		t.Fatalf("Failed to reconcile services: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Test different service name resolution formats
	testCases := []struct {
		name string
		host string
	}{
		{
			name: "service with namespace",
			host: "api-service.production",
		},
		{
			name: "service with full domain",
			host: "api-service.production.svc.cluster.local",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tc.host
			
			w := httptest.NewRecorder()
			lb.HandleRequest(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
				return
			}
			
			body, _ := io.ReadAll(w.Body)
			response := string(body)
			
			if response != "service resolved successfully" {
				t.Errorf("Expected 'service resolved successfully', got: %s", response)
			}
		})
	}
}

// Helper functions

func createTestPodWithPort(name, namespace string, labels map[string]string, ready bool, podIP, port string) storage.Resource {
	now := time.Now().UTC().Truncate(time.Second) // Use UTC and truncate to avoid parsing issues
	metadata := types.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		UID:       "pod-" + name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Parse port
	var containerPort int32 = 8080
	if port != "" {
		fmt.Sscanf(port, "%d", &containerPort)
	}

	spec := types.PodSpec{
		Containers: []types.Container{
			{
				Name:  "test-container",
				Image: "test:latest",
				Ports: []types.ContainerPort{
					{
						Name:          "http",
						ContainerPort: containerPort,
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
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestPod(name, namespace string, labels map[string]string, ready bool, podIP string) storage.Resource {
	now := time.Now().UTC().Truncate(time.Second) // Use UTC and truncate to avoid parsing issues
	metadata := types.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		UID:       "pod-" + name,
		CreatedAt: now,
		UpdatedAt: now,
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
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestServiceWithTargetPort(name, namespace string, selector map[string]string, port, targetPort int32) storage.Resource {
	now := time.Now().UTC().Truncate(time.Second) // Use UTC and truncate to avoid parsing issues
	metadata := types.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		UID:       "service-" + name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if targetPort == 0 {
		targetPort = port
	}

	spec := types.ServiceSpec{
		Selector: selector,
		Ports: []types.ServicePort{
			{
				Name:       "http",
				Port:       port,
				TargetPort: targetPort,
				Protocol:   "TCP",
			},
		},
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
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestService(name, namespace string, selector map[string]string) storage.Resource {
	return createTestServiceWithTargetPort(name, namespace, selector, 80, 8080)
}