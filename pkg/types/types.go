package types

import (
	"time"
)

// ObjectMeta contains metadata that all persisted resources must have
type ObjectMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	UID       string            `json:"uid,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitempty"`
}

// LabelSelector represents a label query over a set of resources
type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// ResourceRequirements describes the compute resource requirements
type ResourceRequirements struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs
type ResourceList map[string]string

// ContainerPort represents a network port in a single container
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// Probe describes a health check to be performed against a container
type Probe struct {
	HTTPGet             *HTTPGetAction `json:"httpGet,omitempty"`
	TCPSocket           *TCPSocketAction `json:"tcpSocket,omitempty"`
	Exec                *ExecAction    `json:"exec,omitempty"`
	InitialDelaySeconds int32          `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32          `json:"periodSeconds,omitempty"`
	TimeoutSeconds      int32          `json:"timeoutSeconds,omitempty"`
	FailureThreshold    int32          `json:"failureThreshold,omitempty"`
}

// HTTPGetAction describes an action based on HTTP Get requests
type HTTPGetAction struct {
	Path   string `json:"path,omitempty"`
	Port   int32  `json:"port"`
	Scheme string `json:"scheme,omitempty"`
}

// TCPSocketAction describes an action based on opening a socket
type TCPSocketAction struct {
	Port int32 `json:"port"`
}

// ExecAction describes a "run in container" action
type ExecAction struct {
	Command []string `json:"command,omitempty"`
}

// Pod represents a collection of containers that can run on a host
type Pod struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Metadata   ObjectMeta `json:"metadata"`
	Spec       PodSpec   `json:"spec"`
	Status     PodStatus `json:"status,omitempty"`
}

// PodSpec is a description of a pod
type PodSpec struct {
	Containers    []Container       `json:"containers"`
	RestartPolicy string            `json:"restartPolicy,omitempty"`
	NodeSelector  map[string]string `json:"nodeSelector,omitempty"`
	NodeName      string            `json:"nodeName,omitempty"`
}

// Container represents a single container that is run within a pod
type Container struct {
	Name           string               `json:"name"`
	Image          string               `json:"image"`
	Ports          []ContainerPort      `json:"ports,omitempty"`
	Resources      ResourceRequirements `json:"resources,omitempty"`
	LivenessProbe  *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe               `json:"readinessProbe,omitempty"`
	Env            []EnvVar             `json:"env,omitempty"`
}

// EnvVar represents an environment variable present in a Container
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PodStatus represents information about the status of a pod
type PodStatus struct {
	Phase             string             `json:"phase,omitempty"`
	Conditions        []PodCondition     `json:"conditions,omitempty"`
	ContainerStatuses []ContainerStatus  `json:"containerStatuses,omitempty"`
	PodIP             string             `json:"podIP,omitempty"`
	StartTime         *time.Time         `json:"startTime,omitempty"`
}

// PodCondition contains details for the current condition of this pod
type PodCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// ContainerStatus contains details for the current status of this container
type ContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	Image        string `json:"image"`
	ImageID      string `json:"imageID"`
	ContainerID  string `json:"containerID,omitempty"`
	State        ContainerState `json:"state,omitempty"`
}

// ContainerState holds a possible state of container
type ContainerState struct {
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty"`
	Running    *ContainerStateRunning    `json:"running,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
}

// ContainerStateWaiting is a waiting state of a container
type ContainerStateWaiting struct {
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ContainerStateRunning is a running state of a container
type ContainerStateRunning struct {
	StartedAt time.Time `json:"startedAt,omitempty"`
}

// ContainerStateTerminated is a terminated state of a container
type ContainerStateTerminated struct {
	ExitCode    int32     `json:"exitCode"`
	Signal      int32     `json:"signal,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	Message     string    `json:"message,omitempty"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	FinishedAt  time.Time `json:"finishedAt,omitempty"`
}

// Service represents a named abstraction of software service
type Service struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Metadata   ObjectMeta    `json:"metadata"`
	Spec       ServiceSpec   `json:"spec"`
	Status     ServiceStatus `json:"status,omitempty"`
}

// ServiceSpec describes the attributes that a user creates on a service
type ServiceSpec struct {
	Selector map[string]string `json:"selector"`
	Ports    []ServicePort     `json:"ports"`
	Type     string            `json:"type,omitempty"`
}

// ServicePort contains information on service's port
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
}

// ServiceStatus represents the current status of a service
type ServiceStatus struct {
	LoadBalancer LoadBalancerStatus `json:"loadBalancer,omitempty"`
	Endpoints    []Endpoint         `json:"endpoints,omitempty"`
}

// LoadBalancerStatus represents the status of a load-balancer
type LoadBalancerStatus struct {
	Ingress []LoadBalancerIngress `json:"ingress,omitempty"`
}

// LoadBalancerIngress represents the status of a load-balancer ingress point
type LoadBalancerIngress struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// Endpoint represents a single logical "backend" implementing a service
type Endpoint struct {
	IP       string `json:"ip"`
	Port     int32  `json:"port"`
	Ready    bool   `json:"ready"`
	NodeName string `json:"nodeName,omitempty"`
}

// Deployment enables declarative updates for Pods
type Deployment struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ObjectMeta       `json:"metadata"`
	Spec       DeploymentSpec   `json:"spec"`
	Status     DeploymentStatus `json:"status,omitempty"`
}

// DeploymentSpec is the specification of the desired behavior of the Deployment
type DeploymentSpec struct {
	Replicas int32              `json:"replicas"`
	Selector LabelSelector      `json:"selector"`
	Template PodTemplateSpec    `json:"template"`
	Strategy DeploymentStrategy `json:"strategy,omitempty"`
}

// PodTemplateSpec describes the data a pod should have when created from a template
type PodTemplateSpec struct {
	Metadata ObjectMeta `json:"metadata,omitempty"`
	Spec     PodSpec    `json:"spec"`
}

// DeploymentStrategy describes how to replace existing pods with new ones
type DeploymentStrategy struct {
	Type          string                 `json:"type,omitempty"`
	RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`
}

// RollingUpdateStrategy specifies the strategy used to replace old Pods by new ones
type RollingUpdateStrategy struct {
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
	MaxSurge       int32 `json:"maxSurge,omitempty"`
}

// DeploymentStatus is the most recently observed status of the Deployment
type DeploymentStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	Replicas            int32              `json:"replicas,omitempty"`
	UpdatedReplicas     int32              `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32              `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32              `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32              `json:"unavailableReplicas,omitempty"`
	Conditions          []DeploymentCondition `json:"conditions,omitempty"`
}

// DeploymentCondition describes the state of a deployment at a certain point
type DeploymentCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastUpdateTime     time.Time `json:"lastUpdateTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// Node represents a worker node in the cluster
type Node struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   ObjectMeta `json:"metadata"`
	Spec       NodeSpec   `json:"spec"`
	Status     NodeStatus `json:"status,omitempty"`
}

// NodeSpec describes the attributes that a node is created with
type NodeSpec struct {
	Unschedulable bool   `json:"unschedulable,omitempty"`
	ExternalID    string `json:"externalID,omitempty"`
}

// NodeStatus is information about the current status of a node
type NodeStatus struct {
	Capacity    ResourceList    `json:"capacity,omitempty"`
	Allocatable ResourceList    `json:"allocatable,omitempty"`
	Phase       string          `json:"phase,omitempty"`
	Conditions  []NodeCondition `json:"conditions,omitempty"`
	Addresses   []NodeAddress   `json:"addresses,omitempty"`
	NodeInfo    NodeSystemInfo  `json:"nodeInfo,omitempty"`
}

// NodeCondition contains condition information for a node
type NodeCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastHeartbeatTime  time.Time `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// NodeAddress contains information for the node's address
type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

// NodeSystemInfo is a set of ids/uuids to uniquely identify the node
type NodeSystemInfo struct {
	MachineID               string `json:"machineID"`
	SystemUUID              string `json:"systemUUID"`
	BootID                  string `json:"bootID"`
	KernelVersion           string `json:"kernelVersion"`
	OSImage                 string `json:"osImage"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	Architecture            string `json:"architecture"`
	OperatingSystem         string `json:"operatingSystem"`
}