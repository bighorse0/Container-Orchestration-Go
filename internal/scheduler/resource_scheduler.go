package scheduler

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"mini-k8s-orchestration/pkg/types"
)

// ResourceScheduler implements a resource-aware scheduling algorithm
type ResourceScheduler struct {
	// Base scheduler to extend
	*Scheduler
}

// NewResourceScheduler creates a new resource-aware scheduler
func NewResourceScheduler(baseScheduler *Scheduler) *ResourceScheduler {
	return &ResourceScheduler{
		Scheduler: baseScheduler,
	}
}

// selectNode selects a node for the pod based on available resources
func (s *ResourceScheduler) selectNode(pod *types.Pod, nodes []*types.Node) (*types.Node, error) {
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

	// Calculate pod resource requirements
	cpuRequest, memoryRequest, err := calculatePodResourceRequests(pod)
	if err != nil {
		log.Printf("Warning: Failed to calculate pod resource requests: %v", err)
		// Fall back to round-robin if resource calculation fails
		nodeIndex := hashString(pod.Metadata.Name) % len(eligibleNodes)
		return eligibleNodes[nodeIndex], nil
	}

	// Find the best node based on resource availability
	var bestNode *types.Node
	var bestScore float64 = -1

	for _, node := range eligibleNodes {
		// Check if node has enough resources
		nodeCPU, nodeMemory, err := getNodeAvailableResources(node)
		if err != nil {
			log.Printf("Warning: Failed to get node resources: %v", err)
			continue
		}

		// Skip node if it doesn't have enough resources
		if nodeCPU < cpuRequest || nodeMemory < memoryRequest {
			continue
		}

		// Calculate score based on resource fit
		// We want to balance between using nodes efficiently and leaving room for future pods
		// Lower score is better (less resource pressure)
		cpuScore := float64(cpuRequest) / float64(nodeCPU)
		memoryScore := float64(memoryRequest) / float64(nodeMemory)
		
		// Use the higher pressure resource as the score
		score := math.Max(cpuScore, memoryScore)
		
		// Prefer nodes with less resource pressure
		if bestScore == -1 || score < bestScore {
			bestScore = score
			bestNode = node
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("no node with sufficient resources available")
	}

	return bestNode, nil
}

// calculatePodResourceRequests calculates the total CPU and memory requests for a pod
func calculatePodResourceRequests(pod *types.Pod) (int64, int64, error) {
	var totalCPU int64
	var totalMemory int64

	for _, container := range pod.Spec.Containers {
		// Get CPU request
		if cpuStr, ok := container.Resources.Requests["cpu"]; ok {
			cpu, err := parseCPUResource(cpuStr)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid CPU request: %w", err)
			}
			totalCPU += cpu
		}

		// Get memory request
		if memStr, ok := container.Resources.Requests["memory"]; ok {
			memory, err := parseMemoryResource(memStr)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid memory request: %w", err)
			}
			totalMemory += memory
		}
	}

	return totalCPU, totalMemory, nil
}

// getNodeAvailableResources gets the available CPU and memory resources for a node
func getNodeAvailableResources(node *types.Node) (int64, int64, error) {
	// Get allocatable resources
	cpuStr, ok := node.Status.Allocatable["cpu"]
	if !ok {
		return 0, 0, fmt.Errorf("node has no allocatable CPU")
	}

	memoryStr, ok := node.Status.Allocatable["memory"]
	if !ok {
		return 0, 0, fmt.Errorf("node has no allocatable memory")
	}

	// Parse CPU
	cpu, err := parseCPUResource(cpuStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid CPU allocatable: %w", err)
	}

	// Parse memory
	memory, err := parseMemoryResource(memoryStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid memory allocatable: %w", err)
	}

	return cpu, memory, nil
}

// parseCPUResource parses a CPU resource string (e.g., "0.5", "500m") to millicores
func parseCPUResource(cpuStr string) (int64, error) {
	// Handle millicpu format (e.g., "500m")
	if strings.HasSuffix(cpuStr, "m") {
		milliStr := strings.TrimSuffix(cpuStr, "m")
		milli, err := strconv.ParseInt(milliStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid CPU millicpu value: %s", milliStr)
		}
		return milli, nil
	}

	// Handle decimal format (e.g., "0.5")
	cpu, err := strconv.ParseFloat(cpuStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value: %s", cpuStr)
	}

	// Convert to millicores
	return int64(cpu * 1000), nil
}

// parseMemoryResource parses a memory resource string (e.g., "64Mi", "1Gi") to bytes
func parseMemoryResource(memStr string) (int64, error) {
	// Regular expressions are overkill for this simple parsing
	// Handle suffixes: Ki, Mi, Gi, Ti, K, M, G, T
	
	// No suffix case
	if len(memStr) == 0 {
		return 0, fmt.Errorf("empty memory string")
	}
	
	// Check for suffix
	suffix := ""
	value := memStr
	
	// Check for 2-character suffix (Ki, Mi, Gi, Ti)
	if len(memStr) >= 2 && (strings.HasSuffix(memStr, "Ki") || 
		strings.HasSuffix(memStr, "Mi") || 
		strings.HasSuffix(memStr, "Gi") || 
		strings.HasSuffix(memStr, "Ti")) {
		suffix = memStr[len(memStr)-2:]
		value = memStr[:len(memStr)-2]
	} else if len(memStr) >= 1 { 
		// Check for 1-character suffix (K, M, G, T, B)
		lastChar := memStr[len(memStr)-1:]
		if lastChar == "K" || lastChar == "M" || lastChar == "G" || lastChar == "T" || lastChar == "B" {
			suffix = lastChar
			value = memStr[:len(memStr)-1]
		}
	}
	
	// Parse the numeric part
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", value)
	}
	
	// Apply multiplier based on suffix
	var multiplier int64 = 1
	switch suffix {
	case "Ki":
		multiplier = 1024
	case "Mi":
		multiplier = 1024 * 1024
	case "Gi":
		multiplier = 1024 * 1024 * 1024
	case "Ti":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "K":
		multiplier = 1000
	case "M":
		multiplier = 1000 * 1000
	case "G":
		multiplier = 1000 * 1000 * 1000
	case "T":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "B", "":
		multiplier = 1
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", suffix)
	}
	
	return int64(val * float64(multiplier)), nil
}