package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mini-k8s-orchestration/internal/runtime"
	"mini-k8s-orchestration/pkg/types"
)

// PodManager manages the lifecycle of pods on a node
type PodManager struct {
	containerRuntime runtime.ContainerRuntime
	pods            map[string]*types.Pod // podUID -> Pod
	containerIDs    map[string]string     // containerName -> containerID
	mu              sync.RWMutex
}

// NewPodManager creates a new pod manager
func NewPodManager(containerRuntime runtime.ContainerRuntime) *PodManager {
	return &PodManager{
		containerRuntime: containerRuntime,
		pods:            make(map[string]*types.Pod),
		containerIDs:    make(map[string]string),
	}
}

// SyncPods synchronizes the desired pod state with the actual state
func (pm *PodManager) SyncPods(desiredPods []*types.Pod) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Create a map of desired pods for quick lookup
	desiredPodsMap := make(map[string]*types.Pod)
	for _, pod := range desiredPods {
		desiredPodsMap[pod.Metadata.UID] = pod
	}

	// Find pods to delete (pods that are no longer in the desired state)
	var podsToDelete []string
	for uid := range pm.pods {
		if _, exists := desiredPodsMap[uid]; !exists {
			podsToDelete = append(podsToDelete, uid)
		}
	}

	// Delete pods that are no longer desired
	for _, uid := range podsToDelete {
		existingPod := pm.pods[uid]
		if err := pm.deletePod(existingPod); err != nil {
			log.Printf("Failed to delete pod %s: %v", existingPod.Metadata.Name, err)
		}
		delete(pm.pods, uid)
	}

	// Create or update pods
	for _, pod := range desiredPods {
		existingPod, exists := pm.pods[pod.Metadata.UID]
		if !exists {
			// New pod, create it
			if err := pm.createPod(pod); err != nil {
				log.Printf("Failed to create pod %s: %v", pod.Metadata.Name, err)
				continue
			}
			pm.pods[pod.Metadata.UID] = pod
		} else {
			// Existing pod, check if it needs to be updated
			if podNeedsUpdate(existingPod, pod) {
				// Delete and recreate the pod
				if err := pm.deletePod(existingPod); err != nil {
					log.Printf("Failed to delete pod %s for update: %v", existingPod.Metadata.Name, err)
					continue
				}
				if err := pm.createPod(pod); err != nil {
					log.Printf("Failed to recreate pod %s: %v", pod.Metadata.Name, err)
					continue
				}
				pm.pods[pod.Metadata.UID] = pod
			}
		}
	}

	return nil
}

// createPod creates a new pod
func (pm *PodManager) createPod(pod *types.Pod) error {
	log.Printf("Creating pod %s", pod.Metadata.Name)

	// Convert pod to container specs
	containerSpecs, err := runtime.PodToContainerSpecs(pod)
	if err != nil {
		return fmt.Errorf("failed to convert pod to container specs: %w", err)
	}

	ctx := context.Background()

	// Create and start containers
	for _, spec := range containerSpecs {
		// Pull image
		if err := pm.containerRuntime.PullImage(ctx, spec.Image); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", spec.Image, err)
		}

		// Create container
		containerID, err := pm.containerRuntime.CreateContainer(ctx, spec)
		if err != nil {
			return fmt.Errorf("failed to create container %s: %w", spec.Name, err)
		}

		// Store container ID
		containerKey := fmt.Sprintf("%s-%s", pod.Metadata.UID, spec.Name)
		pm.containerIDs[containerKey] = containerID

		// Start container
		if err := pm.containerRuntime.StartContainer(ctx, containerID); err != nil {
			return fmt.Errorf("failed to start container %s: %w", spec.Name, err)
		}

		log.Printf("Started container %s for pod %s with ID %s", spec.Name, pod.Metadata.Name, containerID)
	}

	return nil
}

// deletePod deletes a pod and its containers
func (pm *PodManager) deletePod(pod *types.Pod) error {
	log.Printf("Deleting pod %s", pod.Metadata.Name)

	ctx := context.Background()

	// Stop and remove all containers in the pod
	for _, container := range pod.Spec.Containers {
		containerKey := fmt.Sprintf("%s-%s", pod.Metadata.UID, container.Name)
		containerID, exists := pm.containerIDs[containerKey]
		if !exists {
			log.Printf("Container %s not found for pod %s", container.Name, pod.Metadata.Name)
			continue
		}

		// Stop container with a timeout
		if err := pm.containerRuntime.StopContainer(ctx, containerID, 30); err != nil {
			log.Printf("Failed to stop container %s: %v", containerID, err)
		}

		// Remove container
		if err := pm.containerRuntime.RemoveContainer(ctx, containerID, true); err != nil {
			log.Printf("Failed to remove container %s: %v", containerID, err)
		}

		// Remove container ID from map
		delete(pm.containerIDs, containerKey)
	}

	return nil
}

// GetPodStatus gets the status of a pod
func (pm *PodManager) GetPodStatus(pod *types.Pod) (*types.PodStatus, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	ctx := context.Background()
	status := &types.PodStatus{
		Phase:      "Running",
		Conditions: []types.PodCondition{},
		StartTime:  &time.Time{},
	}

	// Check all containers in the pod
	containerStatuses := make([]types.ContainerStatus, 0, len(pod.Spec.Containers))
	allRunning := true
	allSucceeded := true

	for _, container := range pod.Spec.Containers {
		containerKey := fmt.Sprintf("%s-%s", pod.Metadata.UID, container.Name)
		containerID, exists := pm.containerIDs[containerKey]
		if !exists {
			allRunning = false
			allSucceeded = false
			containerStatuses = append(containerStatuses, types.ContainerStatus{
				Name:   container.Name,
				Ready:  false,
				State:  types.ContainerState{Waiting: &types.ContainerStateWaiting{Reason: "ContainerCreating"}},
				Image:  container.Image,
			})
			continue
		}

		// Get container status
		containerStatus, err := pm.containerRuntime.GetContainerStatus(ctx, containerID)
		if err != nil {
			log.Printf("Failed to get status for container %s: %v", containerID, err)
			allRunning = false
			allSucceeded = false
			containerStatuses = append(containerStatuses, types.ContainerStatus{
				Name:   container.Name,
				Ready:  false,
				State:  types.ContainerState{Waiting: &types.ContainerStateWaiting{Reason: "ContainerStatusUnknown"}},
				Image:  container.Image,
			})
			continue
		}

		// Convert container status
		var state types.ContainerState
		ready := false

		switch containerStatus.State {
		case "running":
			state = types.ContainerState{
				Running: &types.ContainerStateRunning{
					StartedAt: time.Unix(containerStatus.Started, 0),
				},
			}
			ready = true
		case "exited":
			allRunning = false
			if containerStatus.ExitCode != 0 {
				allSucceeded = false
			}
			state = types.ContainerState{
				Terminated: &types.ContainerStateTerminated{
					ExitCode:   containerStatus.ExitCode,
					FinishedAt: time.Unix(containerStatus.Finished, 0),
					Reason:     "Completed",
				},
			}
		default:
			allRunning = false
			allSucceeded = false
			state = types.ContainerState{
				Waiting: &types.ContainerStateWaiting{
					Reason: containerStatus.State,
				},
			}
		}

		containerStatuses = append(containerStatuses, types.ContainerStatus{
			Name:         container.Name,
			State:        state,
			Ready:        ready,
			RestartCount: containerStatus.RestartCount,
			Image:        containerStatus.Image,
			ImageID:      containerStatus.ImageID,
			ContainerID:  containerStatus.ID,
		})
	}

	// Set pod phase based on container statuses
	if allRunning {
		status.Phase = "Running"
	} else if allSucceeded {
		status.Phase = "Succeeded"
	} else {
		status.Phase = "Pending"
	}

	status.ContainerStatuses = containerStatuses

	return status, nil
}

// podNeedsUpdate checks if a pod needs to be updated
func podNeedsUpdate(oldPod, newPod *types.Pod) bool {
	// Check if the number of containers has changed
	if len(oldPod.Spec.Containers) != len(newPod.Spec.Containers) {
		return true
	}

	// Check if any container has changed
	for i, newContainer := range newPod.Spec.Containers {
		oldContainer := oldPod.Spec.Containers[i]
		if newContainer.Image != oldContainer.Image {
			return true
		}
		// Check environment variables
		if len(newContainer.Env) != len(oldContainer.Env) {
			return true
		}
		for j, newEnv := range newContainer.Env {
			oldEnv := oldContainer.Env[j]
			if newEnv.Name != oldEnv.Name || newEnv.Value != oldEnv.Value {
				return true
			}
		}
	}

	return false
}