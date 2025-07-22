package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPodSerialization(t *testing.T) {
	pod := &Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: PodSpec{
			Containers: []Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      "TCP",
						},
					},
					Resources: ResourceRequirements{
						Requests: ResourceList{
							"memory": "64Mi",
							"cpu":    "250m",
						},
						Limits: ResourceList{
							"memory": "128Mi",
							"cpu":    "500m",
						},
					},
				},
			},
			RestartPolicy: "Always",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("Failed to marshal pod: %v", err)
	}

	// Test JSON deserialization
	var deserializedPod Pod
	err = json.Unmarshal(data, &deserializedPod)
	if err != nil {
		t.Fatalf("Failed to unmarshal pod: %v", err)
	}

	// Verify key fields
	if deserializedPod.Metadata.Name != "test-pod" {
		t.Errorf("Expected name 'test-pod', got '%s'", deserializedPod.Metadata.Name)
	}
	if len(deserializedPod.Spec.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(deserializedPod.Spec.Containers))
	}
	if deserializedPod.Spec.Containers[0].Image != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got '%s'", deserializedPod.Spec.Containers[0].Image)
	}
}

func TestServiceSerialization(t *testing.T) {
	service := &Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata: ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: ServiceSpec{
			Selector: map[string]string{
				"app": "test",
			},
			Ports: []ServicePort{
				{
					Port:       80,
					TargetPort: 8080,
					Protocol:   "TCP",
				},
			},
			Type: "ClusterIP",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("Failed to marshal service: %v", err)
	}

	// Test JSON deserialization
	var deserializedService Service
	err = json.Unmarshal(data, &deserializedService)
	if err != nil {
		t.Fatalf("Failed to unmarshal service: %v", err)
	}

	// Verify key fields
	if deserializedService.Metadata.Name != "test-service" {
		t.Errorf("Expected name 'test-service', got '%s'", deserializedService.Metadata.Name)
	}
	if len(deserializedService.Spec.Ports) != 1 {
		t.Errorf("Expected 1 port, got %d", len(deserializedService.Spec.Ports))
	}
	if deserializedService.Spec.Ports[0].Port != 80 {
		t.Errorf("Expected port 80, got %d", deserializedService.Spec.Ports[0].Port)
	}
}

func TestDeploymentSerialization(t *testing.T) {
	deployment := &Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: DeploymentSpec{
			Replicas: 3,
			Selector: LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: PodSpec{
					Containers: []Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
			Strategy: DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &RollingUpdateStrategy{
					MaxUnavailable: 1,
					MaxSurge:       1,
				},
			},
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(deployment)
	if err != nil {
		t.Fatalf("Failed to marshal deployment: %v", err)
	}

	// Test JSON deserialization
	var deserializedDeployment Deployment
	err = json.Unmarshal(data, &deserializedDeployment)
	if err != nil {
		t.Fatalf("Failed to unmarshal deployment: %v", err)
	}

	// Verify key fields
	if deserializedDeployment.Metadata.Name != "test-deployment" {
		t.Errorf("Expected name 'test-deployment', got '%s'", deserializedDeployment.Metadata.Name)
	}
	if deserializedDeployment.Spec.Replicas != 3 {
		t.Errorf("Expected 3 replicas, got %d", deserializedDeployment.Spec.Replicas)
	}
}

func TestNodeSerialization(t *testing.T) {
	node := &Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: ObjectMeta{
			Name: "test-node",
		},
		Status: NodeStatus{
			Capacity: ResourceList{
				"cpu":    "4",
				"memory": "8Gi",
			},
			Allocatable: ResourceList{
				"cpu":    "3.5",
				"memory": "7Gi",
			},
			Conditions: []NodeCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
				},
			},
			Addresses: []NodeAddress{
				{
					Type:    "InternalIP",
					Address: "192.168.1.100",
				},
			},
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Failed to marshal node: %v", err)
	}

	// Test JSON deserialization
	var deserializedNode Node
	err = json.Unmarshal(data, &deserializedNode)
	if err != nil {
		t.Fatalf("Failed to unmarshal node: %v", err)
	}

	// Verify key fields
	if deserializedNode.Metadata.Name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", deserializedNode.Metadata.Name)
	}
	if deserializedNode.Status.Capacity["cpu"] != "4" {
		t.Errorf("Expected CPU capacity '4', got '%s'", deserializedNode.Status.Capacity["cpu"])
	}
}

func TestValidatePod(t *testing.T) {
	tests := []struct {
		name    string
		pod     *Pod
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid pod",
			pod: &Pod{
				Metadata: ObjectMeta{
					Name: "valid-pod",
				},
				Spec: PodSpec{
					Containers: []Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pod without name",
			pod: &Pod{
				Metadata: ObjectMeta{},
				Spec: PodSpec{
					Containers: []Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "pod without containers",
			pod: &Pod{
				Metadata: ObjectMeta{
					Name: "no-containers",
				},
				Spec: PodSpec{
					Containers: []Container{},
				},
			},
			wantErr: true,
			errMsg:  "at least one container is required",
		},
		{
			name: "container without image",
			pod: &Pod{
				Metadata: ObjectMeta{
					Name: "no-image",
				},
				Spec: PodSpec{
					Containers: []Container{
						{
							Name: "nginx",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "image is required",
		},
		{
			name: "invalid restart policy",
			pod: &Pod{
				Metadata: ObjectMeta{
					Name: "invalid-restart",
				},
				Spec: PodSpec{
					Containers: []Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
					RestartPolicy: "InvalidPolicy",
				},
			},
			wantErr: true,
			errMsg:  "must be one of: Always, OnFailure, Never",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePod(tt.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidatePod() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidateService(t *testing.T) {
	tests := []struct {
		name    string
		service *Service
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid service",
			service: &Service{
				Metadata: ObjectMeta{
					Name: "valid-service",
				},
				Spec: ServiceSpec{
					Ports: []ServicePort{
						{
							Port: 80,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "service without ports",
			service: &Service{
				Metadata: ObjectMeta{
					Name: "no-ports",
				},
				Spec: ServiceSpec{
					Ports: []ServicePort{},
				},
			},
			wantErr: true,
			errMsg:  "at least one port is required",
		},
		{
			name: "service with invalid port",
			service: &Service{
				Metadata: ObjectMeta{
					Name: "invalid-port",
				},
				Spec: ServiceSpec{
					Ports: []ServicePort{
						{
							Port: 0,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateService(tt.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateService() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidateDeployment(t *testing.T) {
	tests := []struct {
		name       string
		deployment *Deployment
		wantErr    bool
		errMsg     string
	}{
		{
			name: "valid deployment",
			deployment: &Deployment{
				Metadata: ObjectMeta{
					Name: "valid-deployment",
				},
				Spec: DeploymentSpec{
					Replicas: 3,
					Template: PodTemplateSpec{
						Spec: PodSpec{
							Containers: []Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deployment with negative replicas",
			deployment: &Deployment{
				Metadata: ObjectMeta{
					Name: "negative-replicas",
				},
				Spec: DeploymentSpec{
					Replicas: -1,
					Template: PodTemplateSpec{
						Spec: PodSpec{
							Containers: []Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeployment(tt.deployment)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateDeployment() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple name", "test", true},
		{"valid name with dash", "test-pod", true},
		{"valid name with dot", "test.example", true},
		{"invalid uppercase", "Test", false},
		{"invalid starting with dash", "-test", false},
		{"invalid ending with dash", "test-", false},
		{"invalid starting with dot", ".test", false},
		{"invalid ending with dot", "test.", false},
		{"empty string", "", false},
		{"too long", strings.Repeat("a", 254), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidName(tt.input); got != tt.want {
				t.Errorf("isValidName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}