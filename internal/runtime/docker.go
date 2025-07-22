package runtime

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// DockerRuntime implements ContainerRuntime using Docker
type DockerRuntime struct {
	client *client.Client
}

// NewDockerRuntime creates a new Docker runtime client
func NewDockerRuntime() (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	
	return &DockerRuntime{
		client: cli,
	}, nil
}

// Close closes the Docker client connection
func (d *DockerRuntime) Close() error {
	return d.client.Close()
}

// CreateContainer creates a new container from the specification
func (d *DockerRuntime) CreateContainer(ctx context.Context, spec *ContainerSpec) (string, error) {
	// Convert port mappings
	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)
	
	for _, port := range spec.Ports {
		containerPort := nat.Port(fmt.Sprintf("%d/%s", port.ContainerPort, strings.ToLower(port.Protocol)))
		exposedPorts[containerPort] = struct{}{}
		
		hostBinding := nat.PortBinding{}
		if port.HostPort > 0 {
			hostBinding.HostPort = strconv.Itoa(int(port.HostPort))
		}
		portBindings[containerPort] = []nat.PortBinding{hostBinding}
	}
	
	// Convert environment variables
	env := make([]string, len(spec.Env))
	for i, envVar := range spec.Env {
		env[i] = fmt.Sprintf("%s=%s", envVar.Name, envVar.Value)
	}
	
	// Convert resource constraints
	resources := container.Resources{}
	if spec.Resources != nil {
		if spec.Resources.MemoryLimit != "" {
			memoryBytes, err := parseMemory(spec.Resources.MemoryLimit)
			if err == nil {
				resources.Memory = memoryBytes
			}
		}
		if spec.Resources.CPULimit != "" {
			cpuQuota, cpuPeriod, err := parseCPU(spec.Resources.CPULimit)
			if err == nil {
				resources.CPUQuota = cpuQuota
				resources.CPUPeriod = cpuPeriod
			}
		}
	}
	
	// Convert restart policy
	restartPolicy := container.RestartPolicy{}
	switch spec.RestartPolicy {
	case "Always":
		restartPolicy.Name = "always"
	case "OnFailure":
		restartPolicy.Name = "on-failure"
	case "Never":
		restartPolicy.Name = "no"
	default:
		restartPolicy.Name = "always"
	}
	
	// Create container configuration
	config := &container.Config{
		Image:        spec.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       spec.Labels,
	}
	
	// Set command and args if provided
	if len(spec.Command) > 0 {
		config.Cmd = append(spec.Command, spec.Args...)
	} else if len(spec.Args) > 0 {
		config.Cmd = spec.Args
	}
	
	hostConfig := &container.HostConfig{
		PortBindings:  portBindings,
		RestartPolicy: restartPolicy,
		Resources:     resources,
		NetworkMode:   container.NetworkMode(spec.NetworkMode),
	}
	
	networkConfig := &network.NetworkingConfig{}
	
	// Create the container
	resp, err := d.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, spec.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	
	return resp.ID, nil
}

// StartContainer starts a container
func (d *DockerRuntime) StartContainer(ctx context.Context, containerID string) error {
	err := d.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerID, err)
	}
	return nil
}

// StopContainer stops a container
func (d *DockerRuntime) StopContainer(ctx context.Context, containerID string, timeout int) error {
	stopTimeout := time.Duration(timeout) * time.Second
	err := d.client.ContainerStop(ctx, containerID, &stopTimeout)
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerID, err)
	}
	return nil
}

// RemoveContainer removes a container
func (d *DockerRuntime) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	err := d.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: force,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}
	return nil
}

// GetContainerStatus gets the status of a container
func (d *DockerRuntime) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}
	
	// Parse created time
	var createdTime int64
	if createdParsed, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
		createdTime = createdParsed.Unix()
	}
	
	status := &ContainerStatus{
		ID:       inspect.ID,
		Name:     strings.TrimPrefix(inspect.Name, "/"),
		State:    inspect.State.Status,
		Status:   inspect.State.Status,
		Image:    inspect.Config.Image,
		ImageID:  inspect.Image,
		Created:  createdTime,
		ExitCode: int32(inspect.State.ExitCode),
		Error:    inspect.State.Error,
	}
	
	if inspect.State.StartedAt != "" {
		if startTime, err := parseDockerTime(inspect.State.StartedAt); err == nil {
			status.Started = startTime.Unix()
		}
	}
	
	if inspect.State.FinishedAt != "" {
		if finishTime, err := parseDockerTime(inspect.State.FinishedAt); err == nil {
			status.Finished = finishTime.Unix()
		}
	}
	
	// Convert port mappings
	for containerPort, bindings := range inspect.NetworkSettings.Ports {
		for _, binding := range bindings {
			hostPort, _ := strconv.Atoi(binding.HostPort)
			port := PortMapping{
				ContainerPort: int32(containerPort.Int()),
				HostPort:      int32(hostPort),
				Protocol:      containerPort.Proto(),
			}
			status.Ports = append(status.Ports, port)
		}
	}
	
	return status, nil
}

// ListContainers lists containers
func (d *DockerRuntime) ListContainers(ctx context.Context, all bool) ([]*ContainerInfo, error) {
	containers, err := d.client.ContainerList(ctx, types.ContainerListOptions{
		All: all,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	
	var result []*ContainerInfo
	for _, c := range containers {
		info := &ContainerInfo{
			ID:      c.ID,
			Names:   c.Names,
			Image:   c.Image,
			ImageID: c.ImageID,
			Command: c.Command,
			Created: c.Created,
			State:   c.State,
			Status:  c.Status,
			Labels:  c.Labels,
		}
		
		// Convert port mappings
		for _, port := range c.Ports {
			portMapping := PortMapping{
				ContainerPort: int32(port.PrivatePort),
				HostPort:      int32(port.PublicPort),
				Protocol:      port.Type,
			}
			info.Ports = append(info.Ports, portMapping)
		}
		
		result = append(result, info)
	}
	
	return result, nil
}

// PullImage pulls a container image
func (d *DockerRuntime) PullImage(ctx context.Context, imageName string) error {
	reader, err := d.client.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()
	
	// Read the output to complete the pull
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("error reading image pull response: %w", err)
	}
	
	return nil
}

// ListImages lists container images
func (d *DockerRuntime) ListImages(ctx context.Context) ([]*ImageInfo, error) {
	images, err := d.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	
	var result []*ImageInfo
	for _, img := range images {
		info := &ImageInfo{
			ID:       img.ID,
			RepoTags: img.RepoTags,
			Created:  img.Created,
			Size:     img.Size,
		}
		result = append(result, info)
	}
	
	return result, nil
}

// GetContainerLogs gets container logs
func (d *DockerRuntime) GetContainerLogs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	}
	
	logs, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for container %s: %w", containerID, err)
	}
	
	return logs, nil
}

// ExecInContainer executes a command in a container
func (d *DockerRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	// Create exec configuration
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	
	// Create exec instance
	execID, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec in container %s: %w", containerID, err)
	}
	
	// Start exec instance
	err = d.client.ContainerExecStart(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("failed to start exec in container %s: %w", containerID, err)
	}
	
	// Wait for exec to complete
	for {
		inspect, err := d.client.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect exec in container %s: %w", containerID, err)
		}
		
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return fmt.Errorf("command exited with code %d", inspect.ExitCode)
			}
			break
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil
}

// Ping checks if Docker daemon is accessible
func (d *DockerRuntime) Ping(ctx context.Context) error {
	_, err := d.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping Docker daemon: %w", err)
	}
	return nil
}

// parseMemory converts memory strings like "512Mi", "1Gi" to bytes
func parseMemory(memStr string) (int64, error) {
	if memStr == "" {
		return 0, nil
	}
	
	// Regular expression to match memory format
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)(Ki|Mi|Gi|Ti|K|M|G|T|B)?$`)
	matches := re.FindStringSubmatch(memStr)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid memory format: %s", memStr)
	}
	
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", matches[1])
	}
	
	unit := "B"
	if len(matches) > 2 && matches[2] != "" {
		unit = matches[2]
	}
	
	multiplier := int64(1)
	switch unit {
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
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}
	
	return int64(value * float64(multiplier)), nil
}

// parseCPU converts CPU strings like "0.5", "500m" to Docker CPU quota and period
func parseCPU(cpuStr string) (int64, int64, error) {
	if cpuStr == "" {
		return 0, 0, nil
	}
	
	const defaultCPUPeriod = 100000 // 100ms in microseconds
	
	// Handle millicpu format (e.g., "500m")
	if strings.HasSuffix(cpuStr, "m") {
		milliStr := strings.TrimSuffix(cpuStr, "m")
		milli, err := strconv.ParseFloat(milliStr, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid CPU millicpu value: %s", milliStr)
		}
		
		// Convert millicpu to quota (1000m = 1 CPU = 100000 quota)
		quota := int64((milli / 1000.0) * float64(defaultCPUPeriod))
		return quota, defaultCPUPeriod, nil
	}
	
	// Handle decimal format (e.g., "0.5")
	cpu, err := strconv.ParseFloat(cpuStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid CPU value: %s", cpuStr)
	}
	
	quota := int64(cpu * float64(defaultCPUPeriod))
	return quota, defaultCPUPeriod, nil
}

// parseDockerTime parses Docker's time format
func parseDockerTime(timeStr string) (time.Time, error) {
	// Docker uses RFC3339Nano format
	return time.Parse(time.RFC3339Nano, timeStr)
}