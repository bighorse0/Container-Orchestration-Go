package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"mini-k8s-orchestration/internal/runtime"
	"mini-k8s-orchestration/pkg/types"
)

// NodeAgent represents the agent running on each node
type NodeAgent struct {
	nodeName        string
	apiServerURL    string
	containerRuntime runtime.ContainerRuntime
	podManager      *PodManager
	heartbeatTicker *time.Ticker
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// Config represents the configuration for the node agent
type Config struct {
	NodeName     string
	APIServerURL string
	DataDir      string
	HeartbeatInterval time.Duration
}

// NewNodeAgent creates a new node agent
func NewNodeAgent(config Config) (*NodeAgent, error) {
	// Create container runtime
	containerRuntime, err := runtime.NewDockerRuntime()
	if err != nil {
		return nil, fmt.Errorf("failed to create container runtime: %w", err)
	}

	// Create pod manager
	podManager := NewPodManager(containerRuntime)

	return &NodeAgent{
		nodeName:        config.NodeName,
		apiServerURL:    config.APIServerURL,
		containerRuntime: containerRuntime,
		podManager:      podManager,
		heartbeatTicker: time.NewTicker(config.HeartbeatInterval),
		stopCh:          make(chan struct{}),
	}, nil
}

// Start starts the node agent
func (a *NodeAgent) Start() error {
	log.Printf("Starting node agent on %s", a.nodeName)

	// Register node with API server
	if err := a.registerNode(); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start heartbeat goroutine
	a.wg.Add(1)
	go a.runHeartbeat()

	// Start pod sync goroutine
	a.wg.Add(1)
	go a.runPodSync()

	return nil
}

// Stop stops the node agent
func (a *NodeAgent) Stop() {
	log.Printf("Stopping node agent on %s", a.nodeName)
	close(a.stopCh)
	a.heartbeatTicker.Stop()
	a.wg.Wait()
}

// registerNode registers the node with the API server
func (a *NodeAgent) registerNode() error {
	// Get node information
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	// Create node object
	node := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name: a.nodeName,
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
			Addresses: []types.NodeAddress{
				{
					Type:    "Hostname",
					Address: hostname,
				},
			},
			NodeInfo: types.NodeSystemInfo{
				MachineID:               "machine-id",
				SystemUUID:              "system-uuid",
				BootID:                  "boot-id",
				KernelVersion:           "kernel-version",
				OSImage:                 "os-image",
				ContainerRuntimeVersion: "docker-version",
				Architecture:            "architecture",
				OperatingSystem:         "operating-system",
			},
		},
	}

	// Register node with API server
	return a.apiClient().RegisterNode(node)
}

// runHeartbeat sends periodic heartbeats to the API server
func (a *NodeAgent) runHeartbeat() {
	defer a.wg.Done()
	log.Printf("Starting heartbeat for node %s", a.nodeName)

	for {
		select {
		case <-a.heartbeatTicker.C:
			if err := a.sendHeartbeat(); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
		case <-a.stopCh:
			log.Printf("Stopping heartbeat for node %s", a.nodeName)
			return
		}
	}
}

// sendHeartbeat sends a heartbeat to the API server
func (a *NodeAgent) sendHeartbeat() error {
	// Get node status
	status, err := a.getNodeStatus()
	if err != nil {
		return fmt.Errorf("failed to get node status: %w", err)
	}

	// Send heartbeat to API server
	return a.apiClient().UpdateNodeStatus(a.nodeName, status)
}

// getNodeStatus gets the current status of the node
func (a *NodeAgent) getNodeStatus() (*types.NodeStatus, error) {
	// Get Docker info
	ctx := context.Background()
	if err := a.containerRuntime.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping container runtime: %w", err)
	}

	// Check if we can list containers
	_, err := a.containerRuntime.ListContainers(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Calculate resource usage (simplified)
	cpuCapacity := "4"
	memoryCapacity := "8Gi"
	cpuAllocatable := "3.5"
	memoryAllocatable := "7Gi"

	// Create node status
	status := &types.NodeStatus{
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
		Capacity: types.ResourceList{
			"cpu":    cpuCapacity,
			"memory": memoryCapacity,
		},
		Allocatable: types.ResourceList{
			"cpu":    cpuAllocatable,
			"memory": memoryAllocatable,
		},
	}

	return status, nil
}

// runPodSync synchronizes pods with the API server
func (a *NodeAgent) runPodSync() {
	defer a.wg.Done()
	log.Printf("Starting pod sync for node %s", a.nodeName)

	syncTicker := time.NewTicker(10 * time.Second)
	defer syncTicker.Stop()

	for {
		select {
		case <-syncTicker.C:
			if err := a.syncPods(); err != nil {
				log.Printf("Failed to sync pods: %v", err)
			}
		case <-a.stopCh:
			log.Printf("Stopping pod sync for node %s", a.nodeName)
			return
		}
	}
}

// syncPods synchronizes the pods running on the node with the API server
func (a *NodeAgent) syncPods() error {
	// Get assigned pods from API server
	assignedPods, err := a.apiClient().GetAssignedPods(a.nodeName)
	if err != nil {
		return fmt.Errorf("failed to get assigned pods: %w", err)
	}

	// Sync pods with pod manager
	return a.podManager.SyncPods(assignedPods)
}

// apiClient returns a client for the API server
func (a *NodeAgent) apiClient() APIClient {
	return NewAPIClient(a.apiServerURL)
}