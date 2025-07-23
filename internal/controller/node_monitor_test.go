package controller

import (
	"encoding/json"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// MockRepository is a mock implementation of storage.Repository for testing
type MockRepository struct {
	nodes          map[string]*types.Node
	pods           map[string]*types.Pod
	podAssignments map[string]string // podID -> nodeID
	resources      map[string]storage.Resource
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		nodes:          make(map[string]*types.Node),
		pods:           make(map[string]*types.Pod),
		podAssignments: make(map[string]string),
		resources:      make(map[string]storage.Resource),
	}
}

// Mock implementation of storage.Repository interface
func (r *MockRepository) CreateResource(resource storage.Resource) error {
	key := resource.Kind + "/" + resource.Namespace + "/" + resource.Name
	r.resources[key] = resource
	return nil
}

func (r *MockRepository) GetResource(kind, namespace, name string) (storage.Resource, error) {
	key := kind + "/" + namespace + "/" + name
	if resource, exists := r.resources[key]; exists {
		return resource, nil
	}
	return storage.Resource{}, storage.ErrResourceNotFound
}

func (r *MockRepository) UpdateResource(resource storage.Resource) error {
	key := resource.Kind + "/" + resource.Namespace + "/" + resource.Name
	if _, exists := r.resources[key]; exists {
		r.resources[key] = resource
		return nil
	}
	return storage.ErrResourceNotFound
}

func (r *MockRepository) DeleteResource(kind, namespace, name string) error {
	key := kind + "/" + namespace + "/" + name
	if _, exists := r.resources[key]; exists {
		delete(r.resources, key)
		return nil
	}
	return storage.ErrResourceNotFound
}

func (r *MockRepository) ListResources(kind, namespace string) ([]storage.Resource, error) {
	var resources []storage.Resource
	for _, resource := range r.resources {
		if resource.Kind == kind && (namespace == "" || resource.Namespace == namespace) {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (r *MockRepository) CreateNode(node *types.Node) error {
	r.nodes[node.Metadata.UID] = node
	return nil
}

func (r *MockRepository) GetNode(name string) (*types.Node, error) {
	for _, node := range r.nodes {
		if node.Metadata.Name == name {
			return node, nil
		}
	}
	return nil, nil
}

func (r *MockRepository) UpdateNode(node *types.Node) error {
	r.nodes[node.Metadata.UID] = node
	return nil
}

func (r *MockRepository) DeleteNode(name string) error {
	for id, node := range r.nodes {
		if node.Metadata.Name == name {
			delete(r.nodes, id)
			return nil
		}
	}
	return nil
}

func (r *MockRepository) ListNodes() ([]*types.Node, error) {
	var nodes []*types.Node
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (r *MockRepository) UpdateNodeHeartbeat(name string) error {
	for _, node := range r.nodes {
		if node.Metadata.Name == name {
			for i, condition := range node.Status.Conditions {
				if condition.Type == "Ready" {
					node.Status.Conditions[i].LastHeartbeatTime = time.Now()
					break
				}
			}
			return nil
		}
	}
	return nil
}

func (r *MockRepository) AssignPodToNode(podID, nodeID string) error {
	r.podAssignments[podID] = nodeID
	return nil
}

func (r *MockRepository) GetPodAssignment(podID string) (*storage.PodAssignment, error) {
	nodeID, exists := r.podAssignments[podID]
	if !exists {
		return nil, nil
	}
	return &storage.PodAssignment{
		PodID:  podID,
		NodeID: nodeID,
		Status: "Pending",
	}, nil
}

func (r *MockRepository) UpdatePodAssignmentStatus(podID, status string) error {
	// Just check if the assignment exists
	_, exists := r.podAssignments[podID]
	if !exists {
		return nil
	}
	return nil
}

func (r *MockRepository) DeletePodAssignment(podID string) error {
	delete(r.podAssignments, podID)
	return nil
}

func (r *MockRepository) ListPodAssignmentsByNode(nodeID string) ([]*storage.PodAssignment, error) {
	var assignments []*storage.PodAssignment
	for podID, nID := range r.podAssignments {
		if nID == nodeID {
			assignments = append(assignments, &storage.PodAssignment{
				PodID:  podID,
				NodeID: nodeID,
				Status: "Pending",
			})
		}
	}
	return assignments, nil
}

func TestNodeMonitor(t *testing.T) {
	// Create mock repository
	repo := NewMockRepository()
	
	// Create node monitor with short timeout for testing
	heartbeatTimeout := 1 * time.Minute
	monitor := NewNodeMonitor(repo, heartbeatTimeout)
	
	// Create test nodes
	healthyNode := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "healthy-node",
			UID:  "healthy-node-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
					Reason:             "NodeReady",
					Message:            "Node is ready",
				},
			},
		},
	}
	
	unhealthyNode := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "unhealthy-node",
			UID:  "unhealthy-node-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastHeartbeatTime:  time.Now().Add(-2 * heartbeatTimeout), // Old heartbeat
					LastTransitionTime: time.Now().Add(-2 * heartbeatTimeout),
					Reason:             "NodeReady",
					Message:            "Node is ready",
				},
			},
		},
	}
	
	// Add nodes to repository
	repo.CreateNode(healthyNode)
	repo.CreateNode(unhealthyNode)
	
	// Create test pod on unhealthy node
	pod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       "test-pod-uid",
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
			NodeName: "unhealthy-node",
		},
		Status: types.PodStatus{
			Phase: "Running",
		},
	}
	
	// Create pod in repository
	metadataJSON, _ := json.Marshal(pod.Metadata)
	specJSON, _ := json.Marshal(pod.Spec)
	statusJSON, _ := json.Marshal(pod.Status)
	
	resource := storage.Resource{
		ID:        pod.Metadata.UID,
		Kind:      "Pod",
		Namespace: pod.Metadata.Namespace,
		Name:      pod.Metadata.Name,
		Metadata:  string(metadataJSON),
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}
	
	repo.CreateResource(resource)
	
	// Assign pod to unhealthy node
	repo.AssignPodToNode(pod.Metadata.UID, unhealthyNode.Metadata.UID)
	
	// Run node health check
	err := monitor.checkNodeHealth()
	if err != nil {
		t.Fatalf("Failed to check node health: %v", err)
	}
	
	// Verify unhealthy node was marked as not ready
	updatedNode, err := repo.GetNode("unhealthy-node")
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}
	
	readyCondition := getNodeCondition(updatedNode, "Ready")
	if readyCondition == nil {
		t.Fatal("Ready condition not found")
	}
	
	if readyCondition.Status != "False" {
		t.Errorf("Expected Ready condition to be False, got %s", readyCondition.Status)
	}
	
	// Verify healthy node was not affected
	healthyNodeUpdated, err := repo.GetNode("healthy-node")
	if err != nil {
		t.Fatalf("Failed to get healthy node: %v", err)
	}
	
	healthyReadyCondition := getNodeCondition(healthyNodeUpdated, "Ready")
	if healthyReadyCondition == nil {
		t.Fatal("Ready condition not found for healthy node")
	}
	
	if healthyReadyCondition.Status != "True" {
		t.Errorf("Expected healthy node Ready condition to be True, got %s", healthyReadyCondition.Status)
	}
	
	// Verify pod was marked for rescheduling
	resources, err := repo.ListResources("Pod", "")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	
	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}
	
	// Parse pod spec and status
	var updatedSpec types.PodSpec
	if err := json.Unmarshal([]byte(resources[0].Spec), &updatedSpec); err != nil {
		t.Fatalf("Failed to unmarshal pod spec: %v", err)
	}
	
	var updatedStatus types.PodStatus
	if err := json.Unmarshal([]byte(resources[0].Status), &updatedStatus); err != nil {
		t.Fatalf("Failed to unmarshal pod status: %v", err)
	}
	
	// Check if pod was marked for rescheduling
	if updatedSpec.NodeName != "" {
		t.Errorf("Expected pod NodeName to be cleared, got %s", updatedSpec.NodeName)
	}
	
	if updatedStatus.Phase != "Failed" {
		t.Errorf("Expected pod Phase to be Failed, got %s", updatedStatus.Phase)
	}
	
	// Check if pod has NodeFailed condition
	nodeFailedCondition := getPodCondition(updatedStatus, "NodeFailed")
	if nodeFailedCondition == nil {
		t.Fatal("NodeFailed condition not found")
	}
	
	if nodeFailedCondition.Status != "True" {
		t.Errorf("Expected NodeFailed condition to be True, got %s", nodeFailedCondition.Status)
	}
}

// Helper function to get node condition by type
func getNodeCondition(node *types.Node, conditionType string) *types.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == conditionType {
			return &node.Status.Conditions[i]
		}
	}
	return nil
}

// Helper function to get pod condition by type
func getPodCondition(status types.PodStatus, conditionType string) *types.PodCondition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return &status.Conditions[i]
		}
	}
	return nil
}

func TestNodeMonitorStartStop(t *testing.T) {
	repo := NewMockRepository()
	monitor := NewNodeMonitor(repo, 1*time.Minute)
	
	// Start monitor
	monitor.Start()
	
	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Stop monitor
	monitor.Stop()
	
	// Test should complete without hanging
}

func TestNodeMonitorWithNoReadyCondition(t *testing.T) {
	repo := NewMockRepository()
	monitor := NewNodeMonitor(repo, 1*time.Minute)
	
	// Create node without Ready condition but with old timestamp to simulate no heartbeat
	node := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "test-node",
			UID:  "test-node-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{}, // No conditions
		},
	}
	
	repo.CreateNode(node)
	
	// Run health check
	err := monitor.checkNodeHealth()
	if err != nil {
		t.Fatalf("Failed to check node health: %v", err)
	}
	
	// Verify Ready condition was added
	updatedNode, err := repo.GetNode("test-node")
	if err != nil {
		t.Fatalf("Failed to get updated node: %v", err)
	}
	
	readyCondition := getNodeCondition(updatedNode, "Ready")
	if readyCondition == nil {
		t.Fatal("Ready condition should have been added")
	}
	
	// Since the condition was just added with current time, it should be marked as healthy (True)
	// The logic marks nodes as healthy if they haven't exceeded the heartbeat timeout
	if readyCondition.Status != "True" {
		t.Errorf("Expected Ready condition status to be True (newly added), got %s", readyCondition.Status)
	}
}