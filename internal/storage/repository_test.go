package storage

import (
	"testing"

	"mini-k8s-orchestration/pkg/types"
)

func setupTestRepository(t *testing.T) Repository {
	tempDir := t.TempDir()
	
	db, err := NewDatabase(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	
	t.Cleanup(func() {
		db.Close()
	})
	
	return NewSQLRepository(db)
}

func TestResourceOperations(t *testing.T) {
	repo := setupTestRepository(t)
	
	// Test data
	resource := Resource{
		ID:        "test-pod-123",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test-pod",
		Spec:      `{"containers":[{"name":"nginx","image":"nginx:latest"}]}`,
		Status:    `{"phase":"Running"}`,
	}
	
	// Test CreateResource
	err := repo.CreateResource(resource)
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}
	
	// Test GetResource
	retrieved, err := repo.GetResource("Pod", "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to get resource: %v", err)
	}
	
	if retrieved.ID != resource.ID {
		t.Errorf("Expected ID %s, got %s", resource.ID, retrieved.ID)
	}
	if retrieved.Kind != resource.Kind {
		t.Errorf("Expected Kind %s, got %s", resource.Kind, retrieved.Kind)
	}
	if retrieved.Name != resource.Name {
		t.Errorf("Expected Name %s, got %s", resource.Name, retrieved.Name)
	}
	
	// Test UpdateResource
	resource.Status = `{"phase":"Succeeded"}`
	err = repo.UpdateResource(resource)
	if err != nil {
		t.Fatalf("Failed to update resource: %v", err)
	}
	
	// Verify update
	updated, err := repo.GetResource("Pod", "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to get updated resource: %v", err)
	}
	
	if updated.Status != resource.Status {
		t.Errorf("Expected Status %s, got %s", resource.Status, updated.Status)
	}
	
	// Test ListResources
	resources, err := repo.ListResources("Pod", "default")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	
	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}
	
	// Test DeleteResource
	err = repo.DeleteResource("Pod", "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to delete resource: %v", err)
	}
	
	// Verify deletion
	_, err = repo.GetResource("Pod", "default", "test-pod")
	if err == nil {
		t.Error("Expected error when getting deleted resource")
	}
}

func TestNodeOperations(t *testing.T) {
	repo := setupTestRepository(t)
	
	// Test data
	node := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "test-node",
			UID:  "node-123",
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
			Conditions: []types.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}
	
	// Test CreateNode
	err := repo.CreateNode(node)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	
	// Test GetNode
	retrieved, err := repo.GetNode("test-node")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	
	if retrieved.Metadata.Name != node.Metadata.Name {
		t.Errorf("Expected Name %s, got %s", node.Metadata.Name, retrieved.Metadata.Name)
	}
	if retrieved.Status.Capacity["cpu"] != node.Status.Capacity["cpu"] {
		t.Errorf("Expected CPU %s, got %s", node.Status.Capacity["cpu"], retrieved.Status.Capacity["cpu"])
	}
	
	// Test UpdateNode
	node.Status.Capacity["cpu"] = "8"
	err = repo.UpdateNode(node)
	if err != nil {
		t.Fatalf("Failed to update node: %v", err)
	}
	
	// Test UpdateNodeHeartbeat
	err = repo.UpdateNodeHeartbeat("test-node")
	if err != nil {
		t.Fatalf("Failed to update node heartbeat: %v", err)
	}
	
	// Test ListNodes
	nodes, err := repo.ListNodes()
	if err != nil {
		t.Fatalf("Failed to list nodes: %v", err)
	}
	
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
	
	// Test DeleteNode
	err = repo.DeleteNode("test-node")
	if err != nil {
		t.Fatalf("Failed to delete node: %v", err)
	}
	
	// Verify deletion
	_, err = repo.GetNode("test-node")
	if err == nil {
		t.Error("Expected error when getting deleted node")
	}
}

func TestPodAssignmentOperations(t *testing.T) {
	repo := setupTestRepository(t)
	
	// Create a test node first
	node := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: "test-node",
			UID:  "node-123",
		},
		Status: types.NodeStatus{
			Addresses: []types.NodeAddress{
				{
					Type:    "InternalIP",
					Address: "192.168.1.100",
				},
			},
		},
	}
	
	err := repo.CreateNode(node)
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}
	
	// Test AssignPodToNode
	podID := "pod-123"
	nodeID := "node-123"
	
	err = repo.AssignPodToNode(podID, nodeID)
	if err != nil {
		t.Fatalf("Failed to assign pod to node: %v", err)
	}
	
	// Test GetPodAssignment
	assignment, err := repo.GetPodAssignment(podID)
	if err != nil {
		t.Fatalf("Failed to get pod assignment: %v", err)
	}
	
	if assignment.PodID != podID {
		t.Errorf("Expected PodID %s, got %s", podID, assignment.PodID)
	}
	if assignment.NodeID != nodeID {
		t.Errorf("Expected NodeID %s, got %s", nodeID, assignment.NodeID)
	}
	if assignment.Status != "Pending" {
		t.Errorf("Expected Status 'Pending', got %s", assignment.Status)
	}
	
	// Test UpdatePodAssignmentStatus
	err = repo.UpdatePodAssignmentStatus(podID, "Running")
	if err != nil {
		t.Fatalf("Failed to update pod assignment status: %v", err)
	}
	
	// Verify update
	updated, err := repo.GetPodAssignment(podID)
	if err != nil {
		t.Fatalf("Failed to get updated pod assignment: %v", err)
	}
	
	if updated.Status != "Running" {
		t.Errorf("Expected Status 'Running', got %s", updated.Status)
	}
	
	// Test ListPodAssignmentsByNode
	assignments, err := repo.ListPodAssignmentsByNode(nodeID)
	if err != nil {
		t.Fatalf("Failed to list pod assignments: %v", err)
	}
	
	if len(assignments) != 1 {
		t.Errorf("Expected 1 assignment, got %d", len(assignments))
	}
	
	// Test DeletePodAssignment
	err = repo.DeletePodAssignment(podID)
	if err != nil {
		t.Fatalf("Failed to delete pod assignment: %v", err)
	}
	
	// Verify deletion
	_, err = repo.GetPodAssignment(podID)
	if err == nil {
		t.Error("Expected error when getting deleted pod assignment")
	}
}

func TestResourceNotFound(t *testing.T) {
	repo := setupTestRepository(t)
	
	// Test getting non-existent resource
	_, err := repo.GetResource("Pod", "default", "non-existent")
	if err == nil {
		t.Error("Expected error when getting non-existent resource")
	}
	
	// Test updating non-existent resource
	resource := Resource{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "non-existent",
		Spec:      "{}",
		Status:    "{}",
	}
	err = repo.UpdateResource(resource)
	if err == nil {
		t.Error("Expected error when updating non-existent resource")
	}
	
	// Test deleting non-existent resource
	err = repo.DeleteResource("Pod", "default", "non-existent")
	if err == nil {
		t.Error("Expected error when deleting non-existent resource")
	}
}