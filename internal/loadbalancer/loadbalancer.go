package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// LoadBalancer manages HTTP traffic routing to service endpoints
type LoadBalancer struct {
	repository storage.Repository
	server     *http.Server
	mu         sync.RWMutex
	services   map[string]*ServiceProxy // service name -> proxy
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// ServiceProxy represents a load balancer proxy for a single service
type ServiceProxy struct {
	Name      string
	Namespace string
	Endpoints []types.Endpoint
	current   int // current endpoint index for round-robin
	mu        sync.RWMutex
}

// NewLoadBalancer creates a new load balancer instance
func NewLoadBalancer(repository storage.Repository) *LoadBalancer {
	return &LoadBalancer{
		repository: repository,
		services:   make(map[string]*ServiceProxy),
		stopCh:     make(chan struct{}),
	}
}

// Start starts the load balancer HTTP server and service watcher
func (lb *LoadBalancer) Start(port int) error {
	log.Printf("Starting load balancer on port %d", port)

	// Create HTTP server with custom handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", lb.handleRequest)

	lb.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start service watcher
	lb.wg.Add(1)
	go lb.watchServices()

	// Start HTTP server
	go func() {
		if err := lb.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Load balancer server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the load balancer
func (lb *LoadBalancer) Stop() error {
	log.Println("Stopping load balancer")
	
	close(lb.stopCh)
	lb.wg.Wait()

	if lb.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return lb.server.Shutdown(ctx)
	}

	return nil
}

// handleRequest handles incoming HTTP requests and routes them to appropriate services
func (lb *LoadBalancer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Extract service name from Host header or path
	serviceName, namespace := lb.extractServiceInfo(r)
	if serviceName == "" {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Get service proxy
	lb.mu.RLock()
	serviceKey := fmt.Sprintf("%s.%s", serviceName, namespace)
	proxy, exists := lb.services[serviceKey]
	lb.mu.RUnlock()

	if !exists {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Get next healthy endpoint
	endpoint := proxy.getNextHealthyEndpoint()
	if endpoint == nil {
		http.Error(w, "No healthy endpoints available", http.StatusServiceUnavailable)
		return
	}

	// Create reverse proxy to the endpoint
	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", endpoint.IP, endpoint.Port),
	}

	proxy_handler := httputil.NewSingleHostReverseProxy(target)
	
	// Customize the proxy to handle errors
	proxy_handler.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for %s: %v", target.Host, err)
		http.Error(w, "Service temporarily unavailable", http.StatusBadGateway)
	}

	// Forward the request
	proxy_handler.ServeHTTP(w, r)
}

// extractServiceInfo extracts service name and namespace from the request
func (lb *LoadBalancer) extractServiceInfo(r *http.Request) (string, string) {
	// Try to extract from Host header (e.g., "service-name.namespace.svc.cluster.local")
	host := r.Host
	if host != "" {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			serviceName := parts[0]
			namespace := parts[1]
			return serviceName, namespace
		}
		// If no namespace in host, assume default
		if len(parts) == 1 {
			return parts[0], "default"
		}
	}

	// Try to extract from path (e.g., "/service-name/...")
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0], "default"
		}
	}

	return "", ""
}

// watchServices watches for service changes and updates the load balancer configuration
func (lb *LoadBalancer) watchServices() {
	defer lb.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := lb.updateServices(); err != nil {
				log.Printf("Error updating services: %v", err)
			}
		case <-lb.stopCh:
			log.Println("Service watcher stopped")
			return
		}
	}
}

// UpdateServices updates the load balancer configuration with current services (public for testing)
func (lb *LoadBalancer) UpdateServices() error {
	return lb.updateServices()
}

// updateServices updates the load balancer configuration with current services
func (lb *LoadBalancer) updateServices() error {
	// Get all services from repository
	services, err := lb.repository.ListResources("Service", "")
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Track current services to detect deletions
	currentServices := make(map[string]bool)

	// Update or create service proxies
	for _, serviceResource := range services {
		serviceKey := fmt.Sprintf("%s.%s", serviceResource.Name, serviceResource.Namespace)
		currentServices[serviceKey] = true

		// Parse service status to get endpoints
		var status types.ServiceStatus
		if serviceResource.Status != "" {
			if err := json.Unmarshal([]byte(serviceResource.Status), &status); err != nil {
				log.Printf("Failed to unmarshal service status for %s: %v", serviceKey, err)
				continue
			}
		}

		// Update or create service proxy
		if proxy, exists := lb.services[serviceKey]; exists {
			// Update existing proxy
			proxy.updateEndpoints(status.Endpoints)
		} else {
			// Create new proxy
			lb.services[serviceKey] = &ServiceProxy{
				Name:      serviceResource.Name,
				Namespace: serviceResource.Namespace,
				Endpoints: status.Endpoints,
				current:   0,
			}
			log.Printf("Added service proxy for %s", serviceKey)
		}
	}

	// Remove deleted services
	for serviceKey := range lb.services {
		if !currentServices[serviceKey] {
			delete(lb.services, serviceKey)
			log.Printf("Removed service proxy for %s", serviceKey)
		}
	}

	return nil
}

// getNextHealthyEndpoint returns the next healthy endpoint using round-robin algorithm
func (sp *ServiceProxy) getNextHealthyEndpoint() *types.Endpoint {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if len(sp.Endpoints) == 0 {
		return nil
	}

	// Find healthy endpoints
	var healthyEndpoints []types.Endpoint
	for _, endpoint := range sp.Endpoints {
		if endpoint.Ready {
			healthyEndpoints = append(healthyEndpoints, endpoint)
		}
	}

	if len(healthyEndpoints) == 0 {
		return nil
	}

	// Round-robin selection
	endpoint := &healthyEndpoints[sp.current%len(healthyEndpoints)]
	sp.current++

	return endpoint
}

// updateEndpoints updates the endpoints for this service proxy
func (sp *ServiceProxy) updateEndpoints(endpoints []types.Endpoint) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.Endpoints = endpoints
	// Reset current index if it's out of bounds
	if sp.current >= len(endpoints) {
		sp.current = 0
	}
}

// GetServiceEndpoints returns the current endpoints for a service (for testing)
func (lb *LoadBalancer) GetServiceEndpoints(serviceName, namespace string) []types.Endpoint {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	serviceKey := fmt.Sprintf("%s.%s", serviceName, namespace)
	if proxy, exists := lb.services[serviceKey]; exists {
		proxy.mu.RLock()
		defer proxy.mu.RUnlock()
		return append([]types.Endpoint{}, proxy.Endpoints...)
	}

	return nil
}

// HandleRequest handles an HTTP request for testing purposes
func (lb *LoadBalancer) HandleRequest(w http.ResponseWriter, r *http.Request) {
	lb.handleRequest(w, r)
}