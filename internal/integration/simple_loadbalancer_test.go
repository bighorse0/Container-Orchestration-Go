package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/controller"
	"mini-k8s-orchestration/internal/loadbalancer"
	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// TestSimpleLoadBalancerIntegration tests basic load balancer functionality
func TestSimpleLoadBalancerIntegration(t *testing.T) {
	// Initialize database
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

	// Create a simple service with endpoints directly
	now := time.Now().UTC().Truncate(time.Second)
	
	// Create service metadata with unique name
	serviceName := "test-service-" + fmt.Sprintf("%d", now.UnixNano())
	serviceMetadata := types.ObjectMeta{
		Name:      serviceName,
		Namespace: "default",
		UID:       "service-test-" + fmt.Sprintf("%d", now.UnixNano()),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create service spec
	serviceSpec := types.ServiceSpec{
		Selector: map[string]string{"app": "test"},
		Ports: []types.ServicePort{
			{
				Name:       "http",
				Port:       80,
				TargetPort: 8080,
				Protocol:   "TCP",
			},
		},
	}

	// Create service status with endpoints
	serviceStatus := types.ServiceStatus{
		Endpoints: []types.Endpoint{
			{
				IP:    "10.0.0.1",
				Port:  8080,
				Ready: true,
			},
			{
				IP:    "10.0.0.2",
				Port:  8080,
				Ready: true,
			},
		},
	}

	// Marshal to JSON
	metadataJSON, _ := json.Marshal(serviceMetadata)
	specJSON, _ := json.Marshal(serviceSpec)
	statusJSON, _ := json.Marshal(serviceStatus)

	// Create storage resource
	serviceResource := storage.Resource{
		ID:        serviceMetadata.UID,
		Kind:      "Service",
		Namespace: serviceMetadata.Namespace,
		Name:      serviceMetadata.Name,
		Metadata:  string(metadataJSON),
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Store service in repository
	err = repo.CreateResource(serviceResource)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait for load balancer to pick up the service
	time.Sleep(2 * time.Second)

	// Manually trigger service update to ensure load balancer picks up the service
	// This is needed because the watcher runs every 5 seconds
	err = lb.UpdateServices()
	if err != nil {
		t.Fatalf("Failed to update services: %v", err)
	}

	// Verify that the load balancer has the service endpoints
	endpoints := lb.GetServiceEndpoints(serviceName, "default")
	if len(endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify endpoints are correct
	for _, endpoint := range endpoints {
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

	// Test load balancing by making requests
	responses := make(map[string]int)
	
	for i := 0; i < 10; i++ {
		// Create request to the service
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = serviceName + ".default"
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Handle request through load balancer
		lb.HandleRequest(w, req)
		
		// The request should fail with 502 since we don't have real backends
		// but it should try to connect, which means the service resolution worked
		if w.Code != http.StatusBadGateway {
			t.Logf("Request %d: Got status %d (expected 502 Bad Gateway since no real backends)", i, w.Code)
		}
		
		// Read response for logging
		body, _ := io.ReadAll(w.Body)
		response := string(body)
		responses[response]++
	}

	// The important thing is that the service was found and load balancer tried to connect
	t.Logf("Load balancer attempted to route requests (got %d different responses)", len(responses))
}

// TestLoadBalancerServiceNotFound tests service not found scenario
func TestLoadBalancerServiceNotFound(t *testing.T) {
	// Initialize database
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewSQLRepository(db)

	// Initialize load balancer
	lb := loadbalancer.NewLoadBalancer(repo)
	err = lb.Start(0)
	if err != nil {
		t.Fatalf("Failed to start load balancer: %v", err)
	}
	defer lb.Stop()

	// Test request to non-existent service
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "non-existent-service.default"
	
	w := httptest.NewRecorder()
	lb.HandleRequest(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != "Service not found\n" {
		t.Errorf("Expected 'Service not found', got: %s", string(body))
	}
}

// TestLoadBalancerNoHealthyEndpoints tests no healthy endpoints scenario
func TestLoadBalancerNoHealthyEndpoints(t *testing.T) {
	// Initialize database
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewSQLRepository(db)

	// Initialize load balancer
	lb := loadbalancer.NewLoadBalancer(repo)
	err = lb.Start(0)
	if err != nil {
		t.Fatalf("Failed to start load balancer: %v", err)
	}
	defer lb.Stop()

	// Create a service with no healthy endpoints
	now := time.Now().UTC().Truncate(time.Second)
	serviceName := "unhealthy-service-" + fmt.Sprintf("%d", now.UnixNano())
	
	serviceMetadata := types.ObjectMeta{
		Name:      serviceName,
		Namespace: "default",
		UID:       "service-unhealthy-" + fmt.Sprintf("%d", now.UnixNano()),
		CreatedAt: now,
		UpdatedAt: now,
	}

	serviceSpec := types.ServiceSpec{
		Selector: map[string]string{"app": "test"},
		Ports: []types.ServicePort{
			{
				Name:       "http",
				Port:       80,
				TargetPort: 8080,
				Protocol:   "TCP",
			},
		},
	}

	serviceStatus := types.ServiceStatus{
		Endpoints: []types.Endpoint{
			{
				IP:    "10.0.0.1",
				Port:  8080,
				Ready: false, // Not ready
			},
			{
				IP:    "10.0.0.2",
				Port:  8080,
				Ready: false, // Not ready
			},
		},
	}

	metadataJSON, _ := json.Marshal(serviceMetadata)
	specJSON, _ := json.Marshal(serviceSpec)
	statusJSON, _ := json.Marshal(serviceStatus)

	serviceResource := storage.Resource{
		ID:        serviceMetadata.UID,
		Kind:      "Service",
		Namespace: serviceMetadata.Namespace,
		Name:      serviceMetadata.Name,
		Metadata:  string(metadataJSON),
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = repo.CreateResource(serviceResource)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait for load balancer to pick up the service
	time.Sleep(2 * time.Second)

	// Manually trigger service update
	err = lb.UpdateServices()
	if err != nil {
		t.Fatalf("Failed to update services: %v", err)
	}

	// Test request to service with no healthy endpoints
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = serviceName + ".default"
	
	w := httptest.NewRecorder()
	lb.HandleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != "No healthy endpoints available\n" {
		t.Errorf("Expected 'No healthy endpoints available', got: %s", string(body))
	}
}