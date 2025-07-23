package loadbalancer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// MockRepository implements storage.Repository for testing
type MockRepository struct {
	resources      map[string]storage.Resource
	nodes          map[string]*types.Node
	podAssignments map[string]*storage.PodAssignment
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		resources:      make(map[string]storage.Resource),
		nodes:          make(map[string]*types.Node),
		podAssignments: make(map[string]*storage.PodAssignment),
	}
}

func (m *MockRepository) CreateResource(resource storage.Resource) error {
	key := fmt.Sprintf("%s/%s/%s", resource.Kind, resource.Namespace, resource.Name)
	if _, exists := m.resources[key]; exists {
		return fmt.Errorf("resource already exists")
	}
	m.resources[key] = resource
	return nil
}

func (m *MockRepository) GetResource(kind, namespace, name string) (storage.Resource, error) {
	key := fmt.Sprintf("%s/%s/%s", kind, namespace, name)
	if resource, exists := m.resources[key]; exists {
		return resource, nil
	}
	return storage.Resource{}, fmt.Errorf("resource not found")
}

func (m *MockRepository) UpdateResource(resource storage.Resource) error {
	key := fmt.Sprintf("%s/%s/%s", resource.Kind, resource.Namespace, resource.Name)
	if _, exists := m.resources[key]; !exists {
		return fmt.Errorf("resource not found")
	}
	m.resources[key] = resource
	return nil
}

func (m *MockRepository) DeleteResource(kind, namespace, name string) error {
	key := fmt.Sprintf("%s/%s/%s", kind, namespace, name)
	if _, exists := m.resources[key]; !exists {
		return fmt.Errorf("resource not found")
	}
	delete(m.resources, key)
	return nil
}

func (m *MockRepository) ListResources(kind, namespace string) ([]storage.Resource, error) {
	var resources []storage.Resource
	for key, resource := range m.resources {
		if strings.HasPrefix(key, kind+"/") {
			if namespace == "" || resource.Namespace == namespace {
				resources = append(resources, resource)
			}
		}
	}
	return resources, nil
}

// Node operations
func (m *MockRepository) CreateNode(node *types.Node) error {
	if _, exists := m.nodes[node.Metadata.Name]; exists {
		return fmt.Errorf("node already exists")
	}
	m.nodes[node.Metadata.Name] = node
	return nil
}

func (m *MockRepository) GetNode(name string) (*types.Node, error) {
	if node, exists := m.nodes[name]; exists {
		return node, nil
	}
	return nil, fmt.Errorf("node not found")
}

func (m *MockRepository) UpdateNode(node *types.Node) error {
	if _, exists := m.nodes[node.Metadata.Name]; !exists {
		return fmt.Errorf("node not found")
	}
	m.nodes[node.Metadata.Name] = node
	return nil
}

func (m *MockRepository) DeleteNode(name string) error {
	if _, exists := m.nodes[name]; !exists {
		return fmt.Errorf("node not found")
	}
	delete(m.nodes, name)
	return nil
}

func (m *MockRepository) ListNodes() ([]*types.Node, error) {
	var nodes []*types.Node
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (m *MockRepository) UpdateNodeHeartbeat(name string) error {
	if _, exists := m.nodes[name]; !exists {
		return fmt.Errorf("node not found")
	}
	// In a real implementation, this would update the heartbeat timestamp
	return nil
}

// Pod assignment operations
func (m *MockRepository) AssignPodToNode(podID, nodeID string) error {
	if _, exists := m.podAssignments[podID]; exists {
		return fmt.Errorf("pod assignment already exists")
	}
	m.podAssignments[podID] = &storage.PodAssignment{
		PodID:     podID,
		NodeID:    nodeID,
		Status:    "Pending",
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *MockRepository) GetPodAssignment(podID string) (*storage.PodAssignment, error) {
	if assignment, exists := m.podAssignments[podID]; exists {
		return assignment, nil
	}
	return nil, fmt.Errorf("pod assignment not found")
}

func (m *MockRepository) UpdatePodAssignmentStatus(podID, status string) error {
	if assignment, exists := m.podAssignments[podID]; exists {
		assignment.Status = status
		return nil
	}
	return fmt.Errorf("pod assignment not found")
}

func (m *MockRepository) DeletePodAssignment(podID string) error {
	if _, exists := m.podAssignments[podID]; !exists {
		return fmt.Errorf("pod assignment not found")
	}
	delete(m.podAssignments, podID)
	return nil
}

func (m *MockRepository) ListPodAssignmentsByNode(nodeID string) ([]*storage.PodAssignment, error) {
	var assignments []*storage.PodAssignment
	for _, assignment := range m.podAssignments {
		if assignment.NodeID == nodeID {
			assignments = append(assignments, assignment)
		}
	}
	return assignments, nil
}

// Helper function to create a test service with endpoints
func createTestServiceWithEndpoints(name, namespace string, endpoints []types.Endpoint) storage.Resource {
	metadata := types.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		UID:       "service-" + name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	spec := types.ServiceSpec{
		Selector: map[string]string{"app": name},
		Ports: []types.ServicePort{
			{
				Name:       "http",
				Port:       80,
				TargetPort: 8080,
				Protocol:   "TCP",
			},
		},
	}

	status := types.ServiceStatus{
		Endpoints: endpoints,
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

// Mock HTTP server to simulate backend services
func createMockBackend(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
}

func TestLoadBalancer_extractServiceInfo(t *testing.T) {
	lb := NewLoadBalancer(NewMockRepository())

	tests := []struct {
		name              string
		host              string
		path              string
		expectedService   string
		expectedNamespace string
	}{
		{
			name:              "service with namespace in host",
			host:              "web-service.default.svc.cluster.local",
			path:              "/api/users",
			expectedService:   "web-service",
			expectedNamespace: "default",
		},
		{
			name:              "service without namespace in host",
			host:              "web-service",
			path:              "/api/users",
			expectedService:   "web-service",
			expectedNamespace: "default",
		},
		{
			name:              "service in path",
			host:              "",
			path:              "/web-service/api/users",
			expectedService:   "web-service",
			expectedNamespace: "default",
		},
		{
			name:              "empty host and path",
			host:              "",
			path:              "/",
			expectedService:   "",
			expectedNamespace: "",
		},
		{
			name:              "root path only",
			host:              "",
			path:              "/",
			expectedService:   "",
			expectedNamespace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with a dummy URL to avoid parsing issues
			req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
			// Override the host if specified in test
			if tt.host != "" {
				req.Host = tt.host
			} else {
				req.Host = "" // Clear the default host for empty host tests
			}

			service, namespace := lb.extractServiceInfo(req)
			if service != tt.expectedService {
				t.Errorf("extractServiceInfo() service = %v, want %v", service, tt.expectedService)
			}
			if namespace != tt.expectedNamespace {
				t.Errorf("extractServiceInfo() namespace = %v, want %v", namespace, tt.expectedNamespace)
			}
		})
	}
}

func TestServiceProxy_getNextHealthyEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []types.Endpoint
		expected  []string // Expected IPs in round-robin order
	}{
		{
			name: "all healthy endpoints",
			endpoints: []types.Endpoint{
				{IP: "10.0.0.1", Port: 8080, Ready: true},
				{IP: "10.0.0.2", Port: 8080, Ready: true},
				{IP: "10.0.0.3", Port: 8080, Ready: true},
			},
			expected: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.1"},
		},
		{
			name: "mixed healthy and unhealthy endpoints",
			endpoints: []types.Endpoint{
				{IP: "10.0.0.1", Port: 8080, Ready: true},
				{IP: "10.0.0.2", Port: 8080, Ready: false},
				{IP: "10.0.0.3", Port: 8080, Ready: true},
			},
			expected: []string{"10.0.0.1", "10.0.0.3", "10.0.0.1", "10.0.0.3"},
		},
		{
			name: "no healthy endpoints",
			endpoints: []types.Endpoint{
				{IP: "10.0.0.1", Port: 8080, Ready: false},
				{IP: "10.0.0.2", Port: 8080, Ready: false},
			},
			expected: []string{}, // Should return nil
		},
		{
			name:      "no endpoints",
			endpoints: []types.Endpoint{},
			expected:  []string{}, // Should return nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &ServiceProxy{
				Name:      "test-service",
				Namespace: "default",
				Endpoints: tt.endpoints,
				current:   0,
			}

			for i, expectedIP := range tt.expected {
				endpoint := proxy.getNextHealthyEndpoint()
				if expectedIP == "" {
					if endpoint != nil {
						t.Errorf("getNextHealthyEndpoint() call %d = %v, want nil", i, endpoint)
					}
				} else {
					if endpoint == nil {
						t.Errorf("getNextHealthyEndpoint() call %d = nil, want %s", i, expectedIP)
					} else if endpoint.IP != expectedIP {
						t.Errorf("getNextHealthyEndpoint() call %d = %s, want %s", i, endpoint.IP, expectedIP)
					}
				}
			}
		})
	}
}

func TestServiceProxy_updateEndpoints(t *testing.T) {
	proxy := &ServiceProxy{
		Name:      "test-service",
		Namespace: "default",
		Endpoints: []types.Endpoint{
			{IP: "10.0.0.1", Port: 8080, Ready: true},
		},
		current: 5, // Out of bounds
	}

	newEndpoints := []types.Endpoint{
		{IP: "10.0.0.2", Port: 8080, Ready: true},
		{IP: "10.0.0.3", Port: 8080, Ready: true},
	}

	proxy.updateEndpoints(newEndpoints)

	if len(proxy.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(proxy.Endpoints))
	}

	if proxy.current != 0 {
		t.Errorf("Expected current index to be reset to 0, got %d", proxy.current)
	}

	if proxy.Endpoints[0].IP != "10.0.0.2" {
		t.Errorf("Expected first endpoint IP to be 10.0.0.2, got %s", proxy.Endpoints[0].IP)
	}
}

func TestLoadBalancer_updateServices(t *testing.T) {
	repo := NewMockRepository()
	lb := NewLoadBalancer(repo)

	// Create test services
	endpoints1 := []types.Endpoint{
		{IP: "10.0.0.1", Port: 8080, Ready: true},
		{IP: "10.0.0.2", Port: 8080, Ready: true},
	}
	service1 := createTestServiceWithEndpoints("web-service", "default", endpoints1)
	repo.CreateResource(service1)

	endpoints2 := []types.Endpoint{
		{IP: "10.0.0.3", Port: 8080, Ready: true},
	}
	service2 := createTestServiceWithEndpoints("api-service", "production", endpoints2)
	repo.CreateResource(service2)

	// Update services
	err := lb.updateServices()
	if err != nil {
		t.Fatalf("updateServices() error = %v", err)
	}

	// Check that services were added
	if len(lb.services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(lb.services))
	}

	// Check web-service
	webServiceKey := "web-service.default"
	if proxy, exists := lb.services[webServiceKey]; exists {
		if len(proxy.Endpoints) != 2 {
			t.Errorf("Expected web-service to have 2 endpoints, got %d", len(proxy.Endpoints))
		}
	} else {
		t.Errorf("web-service not found in load balancer")
	}

	// Check api-service
	apiServiceKey := "api-service.production"
	if proxy, exists := lb.services[apiServiceKey]; exists {
		if len(proxy.Endpoints) != 1 {
			t.Errorf("Expected api-service to have 1 endpoint, got %d", len(proxy.Endpoints))
		}
	} else {
		t.Errorf("api-service not found in load balancer")
	}

	// Remove one service and update
	repo.DeleteResource("Service", "production", "api-service")
	err = lb.updateServices()
	if err != nil {
		t.Fatalf("updateServices() error = %v", err)
	}

	// Check that service was removed
	if len(lb.services) != 1 {
		t.Errorf("Expected 1 service after deletion, got %d", len(lb.services))
	}

	if _, exists := lb.services[apiServiceKey]; exists {
		t.Errorf("api-service should have been removed from load balancer")
	}
}

func TestLoadBalancer_GetServiceEndpoints(t *testing.T) {
	repo := NewMockRepository()
	lb := NewLoadBalancer(repo)

	// Create test service
	endpoints := []types.Endpoint{
		{IP: "10.0.0.1", Port: 8080, Ready: true},
		{IP: "10.0.0.2", Port: 8080, Ready: false},
	}
	service := createTestServiceWithEndpoints("test-service", "default", endpoints)
	repo.CreateResource(service)

	// Update services
	lb.updateServices()

	// Test getting endpoints
	retrievedEndpoints := lb.GetServiceEndpoints("test-service", "default")
	if len(retrievedEndpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(retrievedEndpoints))
	}

	// Test non-existent service
	nonExistentEndpoints := lb.GetServiceEndpoints("non-existent", "default")
	if nonExistentEndpoints != nil {
		t.Errorf("Expected nil for non-existent service, got %v", nonExistentEndpoints)
	}
}

func TestLoadBalancer_Integration(t *testing.T) {
	// Create mock backends
	backend1 := createMockBackend("response from backend 1")
	defer backend1.Close()
	
	backend2 := createMockBackend("response from backend 2")
	defer backend2.Close()

	// Extract host and port from test servers
	backend1URL := backend1.URL[7:] // Remove "http://"
	backend2URL := backend2.URL[7:] // Remove "http://"
	
	backend1Parts := strings.Split(backend1URL, ":")
	backend2Parts := strings.Split(backend2URL, ":")

	// Create repository and load balancer
	repo := NewMockRepository()
	lb := NewLoadBalancer(repo)

	// Create service with endpoints pointing to mock backends
	endpoints := []types.Endpoint{
		{IP: backend1Parts[0], Port: parseInt32(backend1Parts[1]), Ready: true},
		{IP: backend2Parts[0], Port: parseInt32(backend2Parts[1]), Ready: true},
	}
	service := createTestServiceWithEndpoints("test-service", "default", endpoints)
	repo.CreateResource(service)

	// Update services in load balancer
	lb.updateServices()

	// Test load balancing
	responses := make(map[string]int)
	
	for i := 0; i < 10; i++ {
		// Create request
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "test-service.default"
		
		// Create response recorder
		w := httptest.NewRecorder()
		
		// Handle request
		lb.handleRequest(w, req)
		
		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
			continue
		}
		
		body, _ := io.ReadAll(w.Body)
		response := string(body)
		responses[response]++
	}

	// Verify round-robin distribution
	if len(responses) != 2 {
		t.Errorf("Expected responses from 2 backends, got %d", len(responses))
	}

	// Each backend should have received some requests
	for response, count := range responses {
		if count == 0 {
			t.Errorf("Backend with response '%s' received no requests", response)
		}
	}
}

func TestLoadBalancer_NoHealthyEndpoints(t *testing.T) {
	repo := NewMockRepository()
	lb := NewLoadBalancer(repo)

	// Create service with no healthy endpoints
	endpoints := []types.Endpoint{
		{IP: "10.0.0.1", Port: 8080, Ready: false},
		{IP: "10.0.0.2", Port: 8080, Ready: false},
	}
	service := createTestServiceWithEndpoints("unhealthy-service", "default", endpoints)
	repo.CreateResource(service)

	// Update services
	lb.updateServices()

	// Test request to service with no healthy endpoints
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "unhealthy-service.default"
	
	w := httptest.NewRecorder()
	lb.handleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "No healthy endpoints available") {
		t.Errorf("Expected 'No healthy endpoints available' in response, got: %s", string(body))
	}
}

func TestLoadBalancer_ServiceNotFound(t *testing.T) {
	repo := NewMockRepository()
	lb := NewLoadBalancer(repo)

	// Test request to non-existent service
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "non-existent-service.default"
	
	w := httptest.NewRecorder()
	lb.handleRequest(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "Service not found") {
		t.Errorf("Expected 'Service not found' in response, got: %s", string(body))
	}
}

// Helper function to parse int32 from string
func parseInt32(s string) int32 {
	var result int32
	fmt.Sscanf(s, "%d", &result)
	return result
}