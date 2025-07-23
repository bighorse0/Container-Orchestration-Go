package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mini-k8s-orchestration/internal/api"
	"mini-k8s-orchestration/internal/controller"
	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

func TestNodeRegistrationAndHeartbeat(t *testing.T) {
	// Setup test database
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	repo := storage.NewSQLRepository(db)
	server := api.NewServer(repo, 8080)

	// Start node monitor with short timeout for testing
	nodeMonitor := controller.NewNodeMonitor(repo, 30*time.Second)
	nodeMonitor.Start()
	defer nodeMonitor.Stop()

	// Test 1: Register a new node
	nodeName := fmt.Sprintf("test-worker-node-%d", time.Now().UnixNano())
	node := types.Node{
		Metadata: types.ObjectMeta{
			Name: nodeName,
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

	// Register node
	req, err := http.NewRequest("POST", "/api/v1/nodes", bytes.NewBuffer(nodeJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}

	var createdNode types.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &createdNode); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if createdNode.Metadata.UID == "" {
		t.Error("Expected node UID to be set")
	}

	// Test 2: Send heartbeat
	statusUpdate := types.NodeStatus{
		Capacity: types.ResourceList{
			"cpu":    "4",
			"memory": "8Gi",
		},
		Allocatable: types.ResourceList{
			"cpu":    "3.8", // Updated available resources
			"memory": "7.2Gi",
		},
		Addresses: []types.NodeAddress{
			{
				Type:    "InternalIP",
				Address: "192.168.1.100",
			},
		},
	}

	statusJSON, err := json.Marshal(statusUpdate)
	if err != nil {
		t.Fatalf("Failed to marshal status update: %v", err)
	}

	// Send heartbeat
	req, err = http.NewRequest("POST", fmt.Sprintf("/api/v1/nodes/%s/heartbeat", nodeName), bytes.NewBuffer(statusJSON))
	if err != nil {
		t.Fatalf("Failed to create heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}

	// Test 3: Verify node status was updated
	req, err = http.NewRequest("GET", fmt.Sprintf("/api/v1/nodes/%s", nodeName), nil)
	if err != nil {
		t.Fatalf("Failed to create get request: %v", err)
	}

	rr = httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}

	var updatedNode types.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &updatedNode); err != nil {
		t.Fatalf("Failed to unmarshal updated node: %v", err)
	}

	// Check if allocatable resources were updated
	if updatedNode.Status.Allocatable["cpu"] != "3.8" {
		t.Errorf("Expected CPU allocatable to be 3.8, got %s", updatedNode.Status.Allocatable["cpu"])
	}

	// Check if Ready condition exists and has recent heartbeat
	readyCondition := getNodeCondition(&updatedNode, "Ready")
	if readyCondition == nil {
		t.Fatal("Ready condition not found")
	}

	if readyCondition.Status != "True" {
		t.Errorf("Expected Ready condition to be True, got %s", readyCondition.Status)
	}

	// Check if heartbeat time is recent (within last 5 seconds)
	if time.Since(readyCondition.LastHeartbeatTime) > 5*time.Second {
		t.Error("Expected heartbeat time to be recent")
	}

	// Test 4: List all nodes
	req, err = http.NewRequest("GET", "/api/v1/nodes", nil)
	if err != nil {
		t.Fatalf("Failed to create list request: %v", err)
	}

	rr = httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
		t.Errorf("Response body: %s", rr.Body.String())
	}

	var nodeList map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &nodeList); err != nil {
		t.Fatalf("Failed to unmarshal node list: %v", err)
	}

	items, ok := nodeList["items"].([]interface{})
	if !ok {
		t.Fatal("Expected 'items' field in node list response")
	}

	if len(items) < 1 {
		t.Errorf("Expected at least 1 node in list, got %d", len(items))
	}

	fmt.Printf("✅ Node registration and heartbeat test completed successfully\n")
	fmt.Printf("   - Node registered with UID: %s\n", createdNode.Metadata.UID)
	fmt.Printf("   - Heartbeat updated successfully\n")
	fmt.Printf("   - Node status: %s\n", readyCondition.Status)
	fmt.Printf("   - Last heartbeat: %v\n", readyCondition.LastHeartbeatTime.Format(time.RFC3339))
}

func TestNodeFailureDetection(t *testing.T) {
	// Setup test database
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	repo := storage.NewSQLRepository(db)

	// Create a node with old heartbeat (simulating failure)
	nodeName := fmt.Sprintf("failing-node-%d", time.Now().UnixNano())
	nodeUID := fmt.Sprintf("failing-node-uid-%d", time.Now().UnixNano())
	node := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: nodeName,
			UID:  nodeUID,
		},
		Status: types.NodeStatus{
			Conditions: []types.NodeCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastHeartbeatTime:  time.Now().Add(-5 * time.Minute), // Old heartbeat
					LastTransitionTime: time.Now().Add(-5 * time.Minute),
					Reason:             "NodeReady",
					Message:            "Node is ready",
				},
			},
			Capacity: types.ResourceList{
				"cpu":    "2",
				"memory": "4Gi",
			},
		},
	}

	if err := repo.CreateNode(node); err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}

	// Create a pod on the failing node
	podName := fmt.Sprintf("test-pod-%d", time.Now().UnixNano())
	podUID := fmt.Sprintf("test-pod-uid-%d", time.Now().UnixNano())
	pod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			UID:       podUID,
		},
		Spec: types.PodSpec{
			Containers: []types.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
			NodeName: nodeName,
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

	if err := repo.CreateResource(resource); err != nil {
		t.Fatalf("Failed to create test pod: %v", err)
	}

	// Assign pod to failing node
	if err := repo.AssignPodToNode(podUID, nodeUID); err != nil {
		t.Fatalf("Failed to assign pod to node: %v", err)
	}

	// Start node monitor with short timeout
	nodeMonitor := controller.NewNodeMonitor(repo, 2*time.Minute)

	// Run health check manually
	if err := nodeMonitor.CheckNodeHealth(); err != nil {
		t.Fatalf("Failed to check node health: %v", err)
	}

	// Verify node was marked as not ready
	updatedNode, err := repo.GetNode(nodeName)
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

	// Verify pod was marked for rescheduling
	resources, err := repo.ListResources("Pod", "")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	// Find our specific pod
	var targetResource *storage.Resource
	for _, resource := range resources {
		if resource.ID == podUID {
			targetResource = &resource
			break
		}
	}

	if targetResource == nil {
		t.Fatalf("Could not find target pod with UID %s", podUID)
	}

	// Parse pod spec and status
	var updatedSpec types.PodSpec
	if err := json.Unmarshal([]byte(targetResource.Spec), &updatedSpec); err != nil {
		t.Fatalf("Failed to unmarshal pod spec: %v", err)
	}

	var updatedStatus types.PodStatus
	if err := json.Unmarshal([]byte(targetResource.Status), &updatedStatus); err != nil {
		t.Fatalf("Failed to unmarshal pod status: %v", err)
	}

	// Check if pod was marked for rescheduling
	if updatedSpec.NodeName != "" {
		t.Errorf("Expected pod NodeName to be cleared for rescheduling, got %s", updatedSpec.NodeName)
	}

	if updatedStatus.Phase != "Failed" {
		t.Errorf("Expected pod Phase to be Failed, got %s", updatedStatus.Phase)
	}

	fmt.Printf("✅ Node failure detection test completed successfully\n")
	fmt.Printf("   - Node marked as not ready: %s\n", readyCondition.Status)
	fmt.Printf("   - Pod marked for rescheduling (NodeName cleared)\n")
	fmt.Printf("   - Pod status: %s\n", updatedStatus.Phase)
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

