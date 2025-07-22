package runtime

import (
	"context"
	"io"

	"mini-k8s-orchestration/pkg/types"
)

// ContainerRuntime defines the interface for container operations
type ContainerRuntime interface {
	// Container lifecycle operations
	CreateContainer(ctx context.Context, spec *ContainerSpec) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout int) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	
	// Container inspection and status
	GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error)
	ListContainers(ctx context.Context, all bool) ([]*ContainerInfo, error)
	
	// Image operations
	PullImage(ctx context.Context, image string) error
	ListImages(ctx context.Context) ([]*ImageInfo, error)
	
	// Container logs and execution
	GetContainerLogs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error)
	ExecInContainer(ctx context.Context, containerID string, cmd []string) error
	
	// Health and connectivity
	Ping(ctx context.Context) error
}

// ContainerSpec represents the specification for creating a container
type ContainerSpec struct {
	Name         string
	Image        string
	Command      []string
	Args         []string
	Env          []EnvVar
	Ports        []PortMapping
	Resources    *ResourceConstraints
	RestartPolicy string
	Labels       map[string]string
	NetworkMode  string
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string
	Value string
}

// PortMapping represents a port mapping from container to host
type PortMapping struct {
	ContainerPort int32
	HostPort      int32
	Protocol      string
}

// ResourceConstraints represents resource limits and requests
type ResourceConstraints struct {
	CPULimit      string // e.g., "0.5" for 0.5 CPU cores
	MemoryLimit   string // e.g., "512Mi" for 512 MiB
	CPURequest    string
	MemoryRequest string
}

// ContainerStatus represents the current status of a container
type ContainerStatus struct {
	ID          string
	Name        string
	State       string // "created", "running", "paused", "restarting", "removing", "exited", "dead"
	Status      string // Human-readable status
	Image       string
	ImageID     string
	Created     int64  // Unix timestamp
	Started     int64  // Unix timestamp
	Finished    int64  // Unix timestamp
	ExitCode    int32
	Error       string
	RestartCount int32
	Ports       []PortMapping
}

// ContainerInfo represents basic information about a container
type ContainerInfo struct {
	ID      string
	Names   []string
	Image   string
	ImageID string
	Command string
	Created int64
	State   string
	Status  string
	Ports   []PortMapping
	Labels  map[string]string
}

// ImageInfo represents information about a container image
type ImageInfo struct {
	ID       string
	RepoTags []string
	Created  int64
	Size     int64
}

// PodToContainerSpecs converts a Kubernetes Pod to container specifications
func PodToContainerSpecs(pod *types.Pod) ([]*ContainerSpec, error) {
	var specs []*ContainerSpec
	
	for _, container := range pod.Spec.Containers {
		spec := &ContainerSpec{
			Name:    container.Name,
			Image:   container.Image,
			Command: []string{}, // Will be set from container.Command if available
			Args:    []string{}, // Will be set from container.Args if available
			Env:     make([]EnvVar, len(container.Env)),
			Ports:   make([]PortMapping, len(container.Ports)),
			Labels: map[string]string{
				"pod.name":      pod.Metadata.Name,
				"pod.namespace": pod.Metadata.Namespace,
				"pod.uid":       pod.Metadata.UID,
				"container.name": container.Name,
			},
			NetworkMode: "bridge",
		}
		
		// Convert environment variables
		for i, env := range container.Env {
			spec.Env[i] = EnvVar{
				Name:  env.Name,
				Value: env.Value,
			}
		}
		
		// Convert port mappings
		for i, port := range container.Ports {
			spec.Ports[i] = PortMapping{
				ContainerPort: port.ContainerPort,
				HostPort:      0, // Let Docker assign random host port
				Protocol:      port.Protocol,
			}
			if spec.Ports[i].Protocol == "" {
				spec.Ports[i].Protocol = "TCP"
			}
		}
		
		// Convert resource constraints
		if container.Resources.Limits != nil || container.Resources.Requests != nil {
			spec.Resources = &ResourceConstraints{}
			
			if container.Resources.Limits != nil {
				if cpu, ok := container.Resources.Limits["cpu"]; ok {
					spec.Resources.CPULimit = cpu
				}
				if memory, ok := container.Resources.Limits["memory"]; ok {
					spec.Resources.MemoryLimit = memory
				}
			}
			
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests["cpu"]; ok {
					spec.Resources.CPURequest = cpu
				}
				if memory, ok := container.Resources.Requests["memory"]; ok {
					spec.Resources.MemoryRequest = memory
				}
			}
		}
		
		// Set restart policy from pod spec
		spec.RestartPolicy = pod.Spec.RestartPolicy
		if spec.RestartPolicy == "" {
			spec.RestartPolicy = "Always"
		}
		
		specs = append(specs, spec)
	}
	
	return specs, nil
}