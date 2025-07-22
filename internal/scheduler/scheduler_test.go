package scheduler

import (
	"encoding/json"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// MockRepository is a mock implementation of storage.Repository for testing
type MockRepository struct {
	pods          map[string]*types.Pod
	nodes         map[string]*types.Node
	podAssignments map[string]string // podID -> nodeID
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		pods:          make(map[string]*types.Pod),
		nodes:         make(map[string]*types.Node),
		podAssignments: make(map[string]string),
	}
}

// Mock implementation of storage.Repository interface
func (r *MockRepository) CreateResource(resource storage.Resource) error {
	return nil
}

func (r *MockRepository) GetResource(kind, namespace, name string) (storage.Resource, error) {
	return storage.Resource{}, nil
}

func (r *MockRepository) UpdateResource(resource storage.Resource) error {
	return nil
}

func (r *MockRepository) DeleteResource(kind, namespace, name string) error {
	return nil
}

func (r *MockRepository) ListResources(kind, namespace string) ([]storage.Resource, error) {
	var resources []storage.Resource
	
	if kind == "Pod" {
		for id, pod := range r.pods {
			spec, _ := json.Marshal(pod.Spec)
			status, _ := json.Marshal(pod.Status)
			
			resources = append(resources, storage.Resource{
				ID:        id,
				Kind:      "Pod",
				Namespace: pod.Metadata.Namespace,
				Name:      pod.Metadata.Name,
				Spec:      string(spec),
				Status:    string(status),
				CreatedAt: pod.Metadata.CreatedAt,
				UpdatedAt: pod.Metadata.UpdatedAt,
			})
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

// Test the basic scheduler
func TestScheduler(t *testing.T) {
	// Create mock repository
	repo := NewMockRepository()
	
	// Create scheduler
	scheduler := NewScheduler(repo)
	
	// Create test nodes
	node1 := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "node-1",
			UID:  "node-1-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: types.ResourceList{
				"cpu":    "4",
				"memory": "8Gi",
			},
		},
	}
	
	node2 := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "node-2",
			UID:  "node-2-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: types.ResourceList{
				"cpu":    "8",
				"memory": "16Gi",
			},
		},
	}
	
	// Add nodes to repository
	repo.CreateNode(node1)
	repo.CreateNode(node2)
	
	// Create test pod
	pod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       "test-pod-uid",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Resources: types.ResourceRequirements{
						Requests: types.ResourceList{
							"cpu":    "500m",
							"memory": "256Mi",
						},
					},
				},
			},
		},
		Status: types.PodStatus{
			Phase: "Pending",
		},
	}
	
	// Add pod to repository
	repo.pods[pod.Metadata.UID] = pod
	
	// Test scheduling
	err := scheduler.schedulePod(pod, []*types.Node{node1, node2})
	if err != nil {
		t.Fatalf("Failed to schedule pod: %v", err)
	}
	
	// Check if pod was assigned to a node
	assignment, err := repo.GetPodAssignment(pod.Metadata.UID)
	if err != nil {
		t.Fatalf("Failed to get pod assignment: %v", err)
	}
	
	if assignment == nil {
		t.Fatal("Pod was not assigned to any node")
	}
	
	// Check if pod spec was updated with node name
	if pod.Spec.NodeName == "" {
		t.Error("Pod spec was not updated with node name")
	}
	
	// Check if pod status was updated
	if pod.Status.Phase != "Scheduled" {
		t.Errorf("Expected pod phase 'Scheduled', got '%s'", pod.Status.Phase)
	}
	
	// Check if pod has PodScheduled condition
	hasScheduledCondition := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "PodScheduled" && condition.Status == "True" {
			hasScheduledCondition = true
			break
		}
	}
	
	if !hasScheduledCondition {
		t.Error("Pod does not have PodScheduled condition")
	}
}

// Test the resource scheduler
func TestResourceScheduler(t *testing.T) {
	// Create mock repository
	repo := NewMockRepository()
	
	// Create base scheduler
	baseScheduler := NewScheduler(repo)
	
	// Create resource scheduler
	scheduler := NewResourceScheduler(baseScheduler)
	
	// Create test nodes with different resource capacities
	node1 := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "small-node",
			UID:  "small-node-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: types.ResourceList{
				"cpu":    "2",
				"memory": "4Gi",
			},
		},
	}
	
	node2 := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "large-node",
			UID:  "large-node-uid",
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: types.ResourceList{
				"cpu":    "8",
				"memory": "16Gi",
			},
		},
	}
	
	// Create test pod with high resource requirements
	highResourcePod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.ObjectMeta{
			Name:      "high-resource-pod",
			Namespace: "default",
			UID:       "high-resource-pod-uid",
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Resources: types.ResourceRequirements{
						Requests: types.ResourceList{
							"cpu":    "4",
							"memory": "8Gi",
						},
					},
				},
			},
		},
	}
	
	// Test node selection
	selectedNode, err := scheduler.selectNode(highResourcePod, []*types.Node{node1, node2})
	if err != nil {
		t.Fatalf("Failed to select node: %v", err)
	}
	
	// The high resource pod should be scheduled on the large node
	if selectedNode.Metadata.Name != "large-node" {
		t.Errorf("Expected pod to be scheduled on 'large-node', got '%s'", selectedNode.Metadata.Name)
	}
	
	// Create test pod with node selector
	nodeSelectorPod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.ObjectMeta{
			Name:      "node-selector-pod",
			Namespace: "default",
			UID:       "node-selector-pod-uid",
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
			NodeSelector: map[string]string{
				"disk": "ssd",
			},
		},
	}
	
	// Add label to node2
	node2.Metadata.Labels = map[string]string{
		"disk": "ssd",
	}
	
	// Test node selection with node selector
	selectedNode, err = scheduler.selectNode(nodeSelectorPod, []*types.Node{node1, node2})
	if err != nil {
		t.Fatalf("Failed to select node: %v", err)
	}
	
	// The pod with node selector should be scheduled on the node with matching labels
	if selectedNode.Metadata.Name != "large-node" {
		t.Errorf("Expected pod to be scheduled on 'large-node', got '%s'", selectedNode.Metadata.Name)
	}
}

// Test resource parsing functions
func TestResourceParsing(t *testing.T) {
	// Test CPU parsing
	testCases := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"500m", 500, false},
		{"0.5", 500, false},
		{"2", 2000, false},
		{"invalid", 0, true},
	}
	
	for _, tc := range testCases {
		result, err := parseCPUResource(tc.input)
		if (err != nil) != tc.hasError {
			t.Errorf("parseCPUResource(%s) error = %v, hasError = %v", tc.input, err, tc.hasError)
			continue
		}
		
		if !tc.hasError && result != tc.expected {
			t.Errorf("parseCPUResource(%s) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
	
	// Test memory parsing
	memoryTestCases := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"256Mi", 256 * 1024 * 1024, false},
		{"1Gi", 1024 * 1024 * 1024, false},
		{"1G", 1000 * 1000 * 1000, false},
		{"512", 512, false},
		{"invalid", 0, true},
	}
	
	for _, tc := range memoryTestCases {
		result, err := parseMemoryResource(tc.input)
		if (err != nil) != tc.hasError {
			t.Errorf("parseMemoryResource(%s) error = %v, hasError = %v", tc.input, err, tc.hasError)
			continue
		}
		
		if !tc.hasError && result != tc.expected {
			t.Errorf("parseMemoryResource(%s) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}