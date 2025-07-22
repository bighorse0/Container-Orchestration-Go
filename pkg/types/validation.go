package types

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidatePod validates a Pod resource
func ValidatePod(pod *Pod) error {
	var errors ValidationErrors

	// Validate metadata
	if errs := validateObjectMeta(&pod.Metadata); errs != nil {
		errors = append(errors, errs...)
	}

	// Validate spec
	if errs := validatePodSpec(&pod.Spec); errs != nil {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// ValidateService validates a Service resource
func ValidateService(service *Service) error {
	var errors ValidationErrors

	// Validate metadata
	if errs := validateObjectMeta(&service.Metadata); errs != nil {
		errors = append(errors, errs...)
	}

	// Validate spec
	if errs := validateServiceSpec(&service.Spec); errs != nil {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// ValidateDeployment validates a Deployment resource
func ValidateDeployment(deployment *Deployment) error {
	var errors ValidationErrors

	// Validate metadata
	if errs := validateObjectMeta(&deployment.Metadata); errs != nil {
		errors = append(errors, errs...)
	}

	// Validate spec
	if errs := validateDeploymentSpec(&deployment.Spec); errs != nil {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// ValidateNode validates a Node resource
func ValidateNode(node *Node) error {
	var errors ValidationErrors

	// Validate metadata
	if errs := validateObjectMeta(&node.Metadata); errs != nil {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validateObjectMeta validates common metadata fields
func validateObjectMeta(meta *ObjectMeta) ValidationErrors {
	var errors ValidationErrors

	// Name is required and must be valid
	if meta.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "name is required",
		})
	} else if !isValidName(meta.Name) {
		errors = append(errors, ValidationError{
			Field:   "metadata.name",
			Message: "name must be a valid DNS subdomain",
		})
	}

	// Namespace must be valid if specified
	if meta.Namespace != "" && !isValidName(meta.Namespace) {
		errors = append(errors, ValidationError{
			Field:   "metadata.namespace",
			Message: "namespace must be a valid DNS subdomain",
		})
	}

	return errors
}

// validatePodSpec validates a PodSpec
func validatePodSpec(spec *PodSpec) ValidationErrors {
	var errors ValidationErrors

	// At least one container is required
	if len(spec.Containers) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.containers",
			Message: "at least one container is required",
		})
	}

	// Validate each container
	for i, container := range spec.Containers {
		if errs := validateContainer(&container, fmt.Sprintf("spec.containers[%d]", i)); errs != nil {
			errors = append(errors, errs...)
		}
	}

	// Validate restart policy
	if spec.RestartPolicy != "" {
		validPolicies := []string{"Always", "OnFailure", "Never"}
		if !contains(validPolicies, spec.RestartPolicy) {
			errors = append(errors, ValidationError{
				Field:   "spec.restartPolicy",
				Message: "must be one of: Always, OnFailure, Never",
			})
		}
	}

	return errors
}

// validateContainer validates a Container
func validateContainer(container *Container, fieldPath string) ValidationErrors {
	var errors ValidationErrors

	// Name is required
	if container.Name == "" {
		errors = append(errors, ValidationError{
			Field:   fieldPath + ".name",
			Message: "name is required",
		})
	} else if !isValidName(container.Name) {
		errors = append(errors, ValidationError{
			Field:   fieldPath + ".name",
			Message: "name must be a valid DNS subdomain",
		})
	}

	// Image is required
	if container.Image == "" {
		errors = append(errors, ValidationError{
			Field:   fieldPath + ".image",
			Message: "image is required",
		})
	}

	// Validate ports
	for i, port := range container.Ports {
		if port.ContainerPort <= 0 || port.ContainerPort > 65535 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.ports[%d].containerPort", fieldPath, i),
				Message: "must be between 1 and 65535",
			})
		}
	}

	return errors
}

// validateServiceSpec validates a ServiceSpec
func validateServiceSpec(spec *ServiceSpec) ValidationErrors {
	var errors ValidationErrors

	// At least one port is required
	if len(spec.Ports) == 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.ports",
			Message: "at least one port is required",
		})
	}

	// Validate each port
	for i, port := range spec.Ports {
		if port.Port <= 0 || port.Port > 65535 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.ports[%d].port", i),
				Message: "must be between 1 and 65535",
			})
		}
		if port.TargetPort != 0 && (port.TargetPort <= 0 || port.TargetPort > 65535) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("spec.ports[%d].targetPort", i),
				Message: "must be between 1 and 65535",
			})
		}
	}

	// Validate service type
	if spec.Type != "" {
		validTypes := []string{"ClusterIP", "NodePort", "LoadBalancer"}
		if !contains(validTypes, spec.Type) {
			errors = append(errors, ValidationError{
				Field:   "spec.type",
				Message: "must be one of: ClusterIP, NodePort, LoadBalancer",
			})
		}
	}

	return errors
}

// validateDeploymentSpec validates a DeploymentSpec
func validateDeploymentSpec(spec *DeploymentSpec) ValidationErrors {
	var errors ValidationErrors

	// Replicas must be non-negative
	if spec.Replicas < 0 {
		errors = append(errors, ValidationError{
			Field:   "spec.replicas",
			Message: "must be non-negative",
		})
	}

	// Validate pod template
	if errs := validatePodSpec(&spec.Template.Spec); errs != nil {
		for _, err := range errs {
			err.Field = "spec.template." + err.Field
			errors = append(errors, err)
		}
	}

	return errors
}

// isValidName checks if a name is a valid DNS subdomain
func isValidName(name string) bool {
	// DNS subdomain: lowercase alphanumeric characters, '-' or '.', 
	// start and end with alphanumeric character
	pattern := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched && len(name) <= 253
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}