package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// ServiceController manages service endpoints and watches for pod changes
type ServiceController struct {
	repository storage.Repository
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewServiceController creates a new service controller
func NewServiceController(repository storage.Repository) *ServiceController {
	return &ServiceController{
		repository: repository,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the service controller
func (sc *ServiceController) Start() {
	log.Println("Starting service controller")
	sc.wg.Add(1)
	go sc.run()
}

// Stop stops the service controller
func (sc *ServiceController) Stop() {
	log.Println("Stopping service controller")
	close(sc.stopCh)
	sc.wg.Wait()
}

// run is the main controller loop
func (sc *ServiceController) run() {
	defer sc.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sc.reconcileServices(); err != nil {
				log.Printf("Error reconciling services: %v", err)
			}
		case <-sc.stopCh:
			log.Println("Service controller stopped")
			return
		}
	}
}

// ReconcileServices reconciles all services (public for testing)
func (sc *ServiceController) ReconcileServices() error {
	return sc.reconcileServices()
}

// reconcileServices reconciles all services with their endpoints
func (sc *ServiceController) reconcileServices() error {
	// Get all services
	services, err := sc.repository.ListResources("Service", "")
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	// Get all pods
	pods, err := sc.repository.ListResources("Pod", "")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Reconcile each service
	for _, serviceResource := range services {
		if err := sc.reconcileService(serviceResource, pods); err != nil {
			log.Printf("Failed to reconcile service %s/%s: %v", 
				serviceResource.Namespace, serviceResource.Name, err)
		}
	}

	return nil
}

// reconcileService reconciles a single service with its endpoints
func (sc *ServiceController) reconcileService(serviceResource storage.Resource, pods []storage.Resource) error {
	// Parse service spec
	var serviceSpec types.ServiceSpec
	if err := json.Unmarshal([]byte(serviceResource.Spec), &serviceSpec); err != nil {
		return fmt.Errorf("failed to unmarshal service spec: %w", err)
	}

	// Parse current service status
	var serviceStatus types.ServiceStatus
	if serviceResource.Status != "" {
		if err := json.Unmarshal([]byte(serviceResource.Status), &serviceStatus); err != nil {
			return fmt.Errorf("failed to unmarshal service status: %w", err)
		}
	}

	// Find matching pods based on label selector
	matchingPods, err := sc.findMatchingPods(serviceSpec.Selector, pods, serviceResource.Namespace)
	if err != nil {
		return fmt.Errorf("failed to find matching pods: %w", err)
	}

	// Build endpoints from matching pods
	endpoints, err := sc.buildEndpoints(matchingPods, serviceSpec.Ports)
	if err != nil {
		return fmt.Errorf("failed to build endpoints: %w", err)
	}

	// Check if endpoints have changed
	if !sc.endpointsEqual(serviceStatus.Endpoints, endpoints) {
		log.Printf("Updating endpoints for service %s/%s: %d endpoints", 
			serviceResource.Namespace, serviceResource.Name, len(endpoints))

		// Update service status with new endpoints
		serviceStatus.Endpoints = endpoints

		// Marshal updated status
		statusJSON, err := json.Marshal(serviceStatus)
		if err != nil {
			return fmt.Errorf("failed to marshal service status: %w", err)
		}

		// Update service in database
		serviceResource.Status = string(statusJSON)
		if err := sc.repository.UpdateResource(serviceResource); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
	}

	return nil
}

// findMatchingPods finds pods that match the service selector
func (sc *ServiceController) findMatchingPods(selector map[string]string, pods []storage.Resource, namespace string) ([]storage.Resource, error) {
	var matchingPods []storage.Resource

	for _, podResource := range pods {
		// Skip pods in different namespaces
		if podResource.Namespace != namespace {
			continue
		}

		// Parse pod metadata to get labels
		var podMetadata types.ObjectMeta
		if err := json.Unmarshal([]byte(podResource.Metadata), &podMetadata); err != nil {
			log.Printf("Failed to unmarshal pod metadata for %s/%s: %v", 
				podResource.Namespace, podResource.Name, err)
			continue
		}

		// Parse pod spec
		var podSpec types.PodSpec
		if err := json.Unmarshal([]byte(podResource.Spec), &podSpec); err != nil {
			log.Printf("Failed to unmarshal pod spec for %s/%s: %v", 
				podResource.Namespace, podResource.Name, err)
			continue
		}

		// Parse pod status to check if it's ready
		var podStatus types.PodStatus
		if podResource.Status != "" {
			if err := json.Unmarshal([]byte(podResource.Status), &podStatus); err != nil {
				log.Printf("Failed to unmarshal pod status for %s/%s: %v", 
					podResource.Namespace, podResource.Name, err)
				continue
			}
		}

		// Create pod object
		pod := &types.Pod{
			Metadata: podMetadata,
			Spec:     podSpec,
			Status:   podStatus,
		}

		// Check if pod matches selector
		if sc.matchesSelector(pod.Metadata.Labels, selector) && sc.isPodReady(pod) {
			matchingPods = append(matchingPods, podResource)
		}
	}

	return matchingPods, nil
}

// matchesSelector checks if pod labels match the service selector
func (sc *ServiceController) matchesSelector(podLabels, selector map[string]string) bool {
	if len(selector) == 0 {
		return false // Empty selector matches nothing
	}

	for key, value := range selector {
		if podLabels[key] != value {
			return false
		}
	}

	return true
}

// isPodReady checks if a pod is ready to receive traffic
func (sc *ServiceController) isPodReady(pod *types.Pod) bool {
	// Check if pod is in Running phase
	if pod.Status.Phase != "Running" {
		return false
	}

	// Check if pod has an IP address
	if pod.Status.PodIP == "" {
		return false
	}

	// Check readiness conditions first
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" {
			return condition.Status == "True"
		}
	}

	// If no explicit readiness condition, check container readiness
	if len(pod.Status.ContainerStatuses) > 0 {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				return false
			}
		}
		return true
	}

	// If no containers reported, consider ready (edge case)
	return true
}

// buildEndpoints builds service endpoints from matching pods
func (sc *ServiceController) buildEndpoints(pods []storage.Resource, servicePorts []types.ServicePort) ([]types.Endpoint, error) {
	var endpoints []types.Endpoint

	for _, podResource := range pods {
		// Parse pod spec and status
		var podSpec types.PodSpec
		if err := json.Unmarshal([]byte(podResource.Spec), &podSpec); err != nil {
			log.Printf("Failed to unmarshal pod spec for %s/%s: %v", 
				podResource.Namespace, podResource.Name, err)
			continue
		}

		var podStatus types.PodStatus
		if podResource.Status != "" {
			if err := json.Unmarshal([]byte(podResource.Status), &podStatus); err != nil {
				log.Printf("Failed to unmarshal pod status for %s/%s: %v", 
					podResource.Namespace, podResource.Name, err)
				continue
			}
		}

		// Skip pods without IP
		if podStatus.PodIP == "" {
			continue
		}

		// Create endpoints for each service port
		for _, servicePort := range servicePorts {
			targetPort := servicePort.TargetPort
			if targetPort == 0 {
				targetPort = servicePort.Port
			}

			// Find matching container port
			containerPort := sc.findContainerPort(podSpec.Containers, targetPort, servicePort.Name)
			if containerPort == 0 {
				containerPort = targetPort // Use target port if no matching container port found
			}

			endpoint := types.Endpoint{
				IP:       podStatus.PodIP,
				Port:     containerPort,
				Ready:    sc.isPodReady(&types.Pod{Spec: podSpec, Status: podStatus}),
				NodeName: podSpec.NodeName,
			}

			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints, nil
}

// findContainerPort finds the container port that matches the target port or name
func (sc *ServiceController) findContainerPort(containers []types.Container, targetPort int32, portName string) int32 {
	for _, container := range containers {
		for _, port := range container.Ports {
			// Match by name first, then by port number
			if portName != "" && port.Name == portName {
				return port.ContainerPort
			}
			if port.ContainerPort == targetPort {
				return port.ContainerPort
			}
		}
	}
	return 0
}

// endpointsEqual compares two endpoint slices for equality
func (sc *ServiceController) endpointsEqual(a, b []types.Endpoint) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for comparison
	aMap := make(map[string]types.Endpoint)
	bMap := make(map[string]types.Endpoint)

	for _, endpoint := range a {
		key := fmt.Sprintf("%s:%d", endpoint.IP, endpoint.Port)
		aMap[key] = endpoint
	}

	for _, endpoint := range b {
		key := fmt.Sprintf("%s:%d", endpoint.IP, endpoint.Port)
		bMap[key] = endpoint
	}

	// Compare maps
	for key, aEndpoint := range aMap {
		bEndpoint, exists := bMap[key]
		if !exists || aEndpoint.Ready != bEndpoint.Ready || aEndpoint.NodeName != bEndpoint.NodeName {
			return false
		}
	}

	return true
}