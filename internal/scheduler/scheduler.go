package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// Scheduler is responsible for assigning pods to nodes
type Scheduler struct {
	repository     storage.Repository
	schedulingLock sync.Mutex
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewScheduler creates a new scheduler
func NewScheduler(repository storage.Repository) *Scheduler {
	return &Scheduler{
		repository: repository,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	log.Println("Starting scheduler")
	s.wg.Add(1)
	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler")
	close(s.stopCh)
	s.wg.Wait()
}

// run is the main scheduling loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.schedulePendingPods(); err != nil {
				log.Printf("Error scheduling pods: %v", err)
			}
		case <-s.stopCh:
			log.Println("Scheduler stopped")
			return
		}
	}
}

// schedulePendingPods schedules all pending pods
func (s *Scheduler) schedulePendingPods() error {
	// Get all pending pods
	pendingPods, err := s.getPendingPods()
	if err != nil {
		return fmt.Errorf("failed to get pending pods: %w", err)
	}

	if len(pendingPods) == 0 {
		return nil
	}

	log.Printf("Found %d pending pods to schedule", len(pendingPods))

	// Get all available nodes
	nodes, err := s.getAvailableNodes()
	if err != nil {
		return fmt.Errorf("failed to get available nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no available nodes for scheduling")
	}

	// Schedule each pod
	for _, pod := range pendingPods {
		if err := s.schedulePod(pod, nodes); err != nil {
			log.Printf("Failed to schedule pod %s/%s: %v", pod.Metadata.Namespace, pod.Metadata.Name, err)
			continue
		}
	}

	return nil
}

// schedulePod schedules a single pod to a node
func (s *Scheduler) schedulePod(pod *types.Pod, nodes []*types.Node) error {
	s.schedulingLock.Lock()
	defer s.schedulingLock.Unlock()

	// Check if pod already has a node assignment
	assignment, err := s.repository.GetPodAssignment(pod.Metadata.UID)
	if err == nil && assignment != nil {
		// Pod is already assigned
		return nil
	}

	// Select a node for the pod
	selectedNode, err := s.selectNode(pod, nodes)
	if err != nil {
		return fmt.Errorf("failed to select node: %w", err)
	}

	// Assign pod to node
	if err := s.repository.AssignPodToNode(pod.Metadata.UID, selectedNode.Metadata.UID); err != nil {
		return fmt.Errorf("failed to assign pod to node: %w", err)
	}

	// Update pod status with node name
	pod.Spec.NodeName = selectedNode.Metadata.Name
	pod.Status.Phase = "Scheduled"

	// Add condition
	now := time.Now()
	pod.Status.Conditions = append(pod.Status.Conditions, types.PodCondition{
		Type:               "PodScheduled",
		Status:             "True",
		LastTransitionTime: now,
		Reason:             "Scheduled",
		Message:            fmt.Sprintf("Successfully assigned %s/%s to %s", pod.Metadata.Namespace, pod.Metadata.Name, selectedNode.Metadata.Name),
	})

	// Convert to storage resource
	specJSON, err := json.Marshal(pod.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal pod spec: %w", err)
	}

	statusJSON, err := json.Marshal(pod.Status)
	if err != nil {
		return fmt.Errorf("failed to marshal pod status: %w", err)
	}

	resource := storage.Resource{
		Kind:      "Pod",
		Namespace: pod.Metadata.Namespace,
		Name:      pod.Metadata.Name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}

	// Update pod in storage
	if err := s.repository.UpdateResource(resource); err != nil {
		return fmt.Errorf("failed to update pod: %w", err)
	}

	log.Printf("Scheduled pod %s/%s to node %s", pod.Metadata.Namespace, pod.Metadata.Name, selectedNode.Metadata.Name)
	return nil
}

// selectNode selects a node for the pod using a simple round-robin algorithm
func (s *Scheduler) selectNode(pod *types.Pod, nodes []*types.Node) (*types.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	// Filter nodes based on node selector
	var eligibleNodes []*types.Node
	if len(pod.Spec.NodeSelector) > 0 {
		for _, node := range nodes {
			if matchNodeSelector(node, pod.Spec.NodeSelector) {
				eligibleNodes = append(eligibleNodes, node)
			}
		}
		if len(eligibleNodes) == 0 {
			return nil, fmt.Errorf("no nodes match node selector")
		}
	} else {
		eligibleNodes = nodes
	}

	// Simple round-robin: use pod name hash to select node
	nodeIndex := hashString(pod.Metadata.Name) % len(eligibleNodes)
	return eligibleNodes[nodeIndex], nil
}

// getPendingPods gets all pods that need to be scheduled
func (s *Scheduler) getPendingPods() ([]*types.Pod, error) {
	// Get all pods
	resources, err := s.repository.ListResources("Pod", "")
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var pendingPods []*types.Pod
	for _, resource := range resources {
		// Parse pod spec and status
		var spec types.PodSpec
		if err := json.Unmarshal([]byte(resource.Spec), &spec); err != nil {
			log.Printf("Failed to unmarshal pod spec: %v", err)
			continue
		}

		var status types.PodStatus
		if resource.Status != "" {
			if err := json.Unmarshal([]byte(resource.Status), &status); err != nil {
				log.Printf("Failed to unmarshal pod status: %v", err)
				continue
			}
		}

		// Check if pod needs scheduling
		if spec.NodeName == "" && status.Phase != "Scheduled" {
			pod := &types.Pod{
				APIVersion: "v1",
				Kind:       "Pod",
				Metadata: types.ObjectMeta{
					Name:      resource.Name,
					Namespace: resource.Namespace,
					UID:       resource.ID,
					CreatedAt: resource.CreatedAt,
					UpdatedAt: resource.UpdatedAt,
				},
				Spec:   spec,
				Status: status,
			}
			pendingPods = append(pendingPods, pod)
		}
	}

	return pendingPods, nil
}

// getAvailableNodes gets all available nodes
func (s *Scheduler) getAvailableNodes() ([]*types.Node, error) {
	// Get all nodes
	nodes, err := s.repository.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var availableNodes []*types.Node
	for _, node := range nodes {
		// Check if node is ready
		if isNodeReady(node) {
			availableNodes = append(availableNodes, node)
		}
	}

	return availableNodes, nil
}

// isNodeReady checks if a node is ready
func isNodeReady(node *types.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

// matchNodeSelector checks if a node matches the node selector
func matchNodeSelector(node *types.Node, selector map[string]string) bool {
	for key, value := range selector {
		if node.Metadata.Labels[key] != value {
			return false
		}
	}
	return true
}

// hashString creates a simple hash of a string
func hashString(s string) int {
	hash := 0
	for i := 0; i < len(s); i++ {
		hash = 31*hash + int(s[i])
	}
	return hash
}