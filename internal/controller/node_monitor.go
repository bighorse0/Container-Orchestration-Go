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

// NodeMonitor monitors node health and handles node failures
type NodeMonitor struct {
	repository       storage.Repository
	heartbeatTimeout time.Duration
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// NewNodeMonitor creates a new node monitor
func NewNodeMonitor(repository storage.Repository, heartbeatTimeout time.Duration) *NodeMonitor {
	return &NodeMonitor{
		repository:       repository,
		heartbeatTimeout: heartbeatTimeout,
		stopCh:           make(chan struct{}),
	}
}

// Start starts the node monitor
func (nm *NodeMonitor) Start() {
	log.Println("Starting node monitor")
	nm.wg.Add(1)
	go nm.run()
}

// Stop stops the node monitor
func (nm *NodeMonitor) Stop() {
	log.Println("Stopping node monitor")
	close(nm.stopCh)
	nm.wg.Wait()
}

// run is the main monitoring loop
func (nm *NodeMonitor) run() {
	defer nm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := nm.checkNodeHealth(); err != nil {
				log.Printf("Error checking node health: %v", err)
			}
		case <-nm.stopCh:
			log.Println("Node monitor stopped")
			return
		}
	}
}

// CheckNodeHealth checks the health of all nodes (public for testing)
func (nm *NodeMonitor) CheckNodeHealth() error {
	return nm.checkNodeHealth()
}

// checkNodeHealth checks the health of all nodes
func (nm *NodeMonitor) checkNodeHealth() error {
	// Get all nodes
	nodes, err := nm.repository.ListNodes()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, node := range nodes {
		// Find the Ready condition
		var readyCondition *types.NodeCondition
		for i := range node.Status.Conditions {
			if node.Status.Conditions[i].Type == "Ready" {
				readyCondition = &node.Status.Conditions[i]
				break
			}
		}

		if readyCondition == nil {
			// No Ready condition found, add one
			node.Status.Conditions = append(node.Status.Conditions, types.NodeCondition{
				Type:               "Ready",
				Status:             "Unknown",
				LastHeartbeatTime:  now,
				LastTransitionTime: now,
				Reason:             "NodeStatusNeverUpdated",
				Message:            "Node has never reported status",
			})
			readyCondition = &node.Status.Conditions[len(node.Status.Conditions)-1]
		}

		// Check if the node has missed heartbeats
		timeSinceHeartbeat := now.Sub(readyCondition.LastHeartbeatTime)
		if timeSinceHeartbeat > nm.heartbeatTimeout {
			// Node is not healthy
			if readyCondition.Status != "False" {
				log.Printf("Node %s is not healthy (last heartbeat: %v ago)", node.Metadata.Name, timeSinceHeartbeat)
				
				// Update node status
				readyCondition.Status = "False"
				readyCondition.LastTransitionTime = now
				readyCondition.Reason = "NodeNotReady"
				readyCondition.Message = fmt.Sprintf("Node has not sent a heartbeat for %v", timeSinceHeartbeat)
				
				// Update node in database
				if err := nm.repository.UpdateNode(node); err != nil {
					log.Printf("Failed to update node status: %v", err)
				}
				
				// Handle node failure
				if err := nm.handleNodeFailure(node); err != nil {
					log.Printf("Failed to handle node failure: %v", err)
				}
			}
		} else if readyCondition.Status != "True" {
			// Node is healthy again
			log.Printf("Node %s is healthy again", node.Metadata.Name)
			
			// Update node status
			readyCondition.Status = "True"
			readyCondition.LastTransitionTime = now
			readyCondition.Reason = "NodeReady"
			readyCondition.Message = "Node is ready"
			
			// Update node in database
			if err := nm.repository.UpdateNode(node); err != nil {
				log.Printf("Failed to update node status: %v", err)
			}
		}
	}

	return nil
}

// handleNodeFailure handles a node failure
func (nm *NodeMonitor) handleNodeFailure(node *types.Node) error {
	// Get pod assignments for this node
	assignments, err := nm.repository.ListPodAssignmentsByNode(node.Metadata.UID)
	if err != nil {
		return err
	}

	log.Printf("Found %d pods on failed node %s", len(assignments), node.Metadata.Name)

	// Mark pods as failed
	for _, assignment := range assignments {
		// Update pod assignment status
		if err := nm.repository.UpdatePodAssignmentStatus(assignment.PodID, "NodeFailed"); err != nil {
			log.Printf("Failed to update pod assignment status: %v", err)
			continue
		}

		// Get all pods
		resources, err := nm.repository.ListResources("Pod", "")
		if err != nil {
			log.Printf("Failed to list pods: %v", err)
			continue
		}

		// Find pod with matching ID
		for _, resource := range resources {
			if resource.ID == assignment.PodID {
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

				// Update pod status
				status.Phase = "Failed"
				status.Conditions = append(status.Conditions, types.PodCondition{
					Type:               "NodeFailed",
					Status:             "True",
					LastTransitionTime: time.Now(),
					Reason:             "NodeNotReady",
					Message:            fmt.Sprintf("Node %s is not ready", node.Metadata.Name),
				})

				// Clear node name to allow rescheduling
				spec.NodeName = ""

				// Update pod in database
				specJSON, err := json.Marshal(spec)
				if err != nil {
					log.Printf("Failed to marshal pod spec: %v", err)
					continue
				}

				statusJSON, err := json.Marshal(status)
				if err != nil {
					log.Printf("Failed to marshal pod status: %v", err)
					continue
				}

				resource.Spec = string(specJSON)
				resource.Status = string(statusJSON)

				if err := nm.repository.UpdateResource(resource); err != nil {
					log.Printf("Failed to update pod: %v", err)
					continue
				}

				log.Printf("Marked pod %s/%s as failed due to node failure", resource.Namespace, resource.Name)
				break
			}
		}
	}

	return nil
}