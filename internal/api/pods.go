package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"mini-k8s-orchestration/internal/storage"
	"mini-k8s-orchestration/pkg/types"
)

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Code    int               `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

// createPod handles POST /api/v1/pods
func (s *Server) createPod(c *gin.Context) {
	s.createPodInNamespace(c, "default")
}

// createNamespacedPod handles POST /api/v1/namespaces/{namespace}/pods
func (s *Server) createNamespacedPod(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.createPodInNamespace(c, namespace)
}

// createPodInNamespace creates a pod in the specified namespace
func (s *Server) createPodInNamespace(c *gin.Context, namespace string) {
	var pod types.Pod
	
	if err := c.ShouldBindJSON(&pod); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set namespace if not provided
	if pod.Metadata.Namespace == "" {
		pod.Metadata.Namespace = namespace
	}
	
	// Validate namespace matches URL parameter
	if pod.Metadata.Namespace != namespace {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAMESPACE_MISMATCH",
			Message: "Pod namespace does not match URL namespace",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the pod
	if err := types.ValidatePod(&pod); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Pod validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set metadata
	now := time.Now()
	pod.APIVersion = "v1"
	pod.Kind = "Pod"
	pod.Metadata.UID = uuid.New().String()
	pod.Metadata.CreatedAt = now
	pod.Metadata.UpdatedAt = now
	
	// Initialize status
	pod.Status = types.PodStatus{
		Phase: "Pending",
		Conditions: []types.PodCondition{
			{
				Type:               "PodScheduled",
				Status:             "False",
				LastTransitionTime: now,
				Reason:             "Unschedulable",
				Message:            "Pod is waiting to be scheduled",
			},
		},
	}
	
	// Convert to storage resource
	specJSON, err := json.Marshal(pod.Spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize pod spec",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	statusJSON, err := json.Marshal(pod.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize pod status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	resource := storage.Resource{
		ID:        pod.Metadata.UID,
		Kind:      "Pod",
		Namespace: pod.Metadata.Namespace,
		Name:      pod.Metadata.Name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}
	
	// Save to database
	if err := s.repository.CreateResource(resource); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "RESOURCE_EXISTS",
			Message: "Pod already exists",
			Code:    http.StatusConflict,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusCreated, pod)
}

// getPod handles GET /api/v1/pods/{name}
func (s *Server) getPod(c *gin.Context) {
	s.getPodFromNamespace(c, "default")
}

// getNamespacedPod handles GET /api/v1/namespaces/{namespace}/pods/{name}
func (s *Server) getNamespacedPod(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.getPodFromNamespace(c, namespace)
}

// getPodFromNamespace gets a pod from the specified namespace
func (s *Server) getPodFromNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Pod name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Get from database
	resource, err := s.repository.GetResource("Pod", namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Pod not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	// Convert to Pod
	pod, err := s.resourceToPod(resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DESERIALIZATION_ERROR",
			Message: "Failed to deserialize pod",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, pod)
}

// updatePod handles PUT /api/v1/pods/{name}
func (s *Server) updatePod(c *gin.Context) {
	s.updatePodInNamespace(c, "default")
}

// updateNamespacedPod handles PUT /api/v1/namespaces/{namespace}/pods/{name}
func (s *Server) updateNamespacedPod(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.updatePodInNamespace(c, namespace)
}

// updatePodInNamespace updates a pod in the specified namespace
func (s *Server) updatePodInNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Pod name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	var pod types.Pod
	if err := c.ShouldBindJSON(&pod); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Validate name matches URL parameter
	if pod.Metadata.Name != name {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAME_MISMATCH",
			Message: "Pod name does not match URL parameter",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate namespace
	if pod.Metadata.Namespace != namespace {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAMESPACE_MISMATCH",
			Message: "Pod namespace does not match URL namespace",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the pod
	if err := types.ValidatePod(&pod); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Pod validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Update timestamp
	pod.Metadata.UpdatedAt = time.Now()
	
	// Convert to storage resource
	specJSON, err := json.Marshal(pod.Spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize pod spec",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	statusJSON, err := json.Marshal(pod.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize pod status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	resource := storage.Resource{
		Kind:      "Pod",
		Namespace: namespace,
		Name:      name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}
	
	// Update in database
	if err := s.repository.UpdateResource(resource); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Pod not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, pod)
}

// deletePod handles DELETE /api/v1/pods/{name}
func (s *Server) deletePod(c *gin.Context) {
	s.deletePodFromNamespace(c, "default")
}

// deleteNamespacedPod handles DELETE /api/v1/namespaces/{namespace}/pods/{name}
func (s *Server) deleteNamespacedPod(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.deletePodFromNamespace(c, namespace)
}

// deletePodFromNamespace deletes a pod from the specified namespace
func (s *Server) deletePodFromNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Pod name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Delete from database
	if err := s.repository.DeleteResource("Pod", namespace, name); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Pod not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Pod deleted successfully",
	})
}

// listPods handles GET /api/v1/pods
func (s *Server) listPods(c *gin.Context) {
	s.listPodsInNamespace(c, "")
}

// listNamespacedPods handles GET /api/v1/namespaces/{namespace}/pods
func (s *Server) listNamespacedPods(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.listPodsInNamespace(c, namespace)
}

// listPodsInNamespace lists pods in the specified namespace
func (s *Server) listPodsInNamespace(c *gin.Context, namespace string) {
	// Get from database
	resources, err := s.repository.ListResources("Pod", namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to list pods",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	// Convert to pods
	var pods []types.Pod
	for _, resource := range resources {
		pod, err := s.resourceToPod(resource)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "DESERIALIZATION_ERROR",
				Message: "Failed to deserialize pod",
				Code:    http.StatusInternalServerError,
				Details: map[string]string{"error": err.Error()},
			})
			return
		}
		pods = append(pods, *pod)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"apiVersion": "v1",
		"kind":       "PodList",
		"items":      pods,
	})
}

// resourceToPod converts a storage resource to a Pod
func (s *Server) resourceToPod(resource storage.Resource) (*types.Pod, error) {
	var spec types.PodSpec
	if err := json.Unmarshal([]byte(resource.Spec), &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pod spec: %w", err)
	}
	
	var status types.PodStatus
	if resource.Status != "" {
		if err := json.Unmarshal([]byte(resource.Status), &status); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pod status: %w", err)
		}
	}
	
	pod := &types.Pod{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: types.ObjectMeta{
			Name:      resource.Name,
			Namespace: resource.Namespace,
			UID:       resource.ID,
			CreatedAt: resource.CreatedAt,
			UpdatedAt: resource.UpdatedAt,
		},
		Spec:   spec,
		Status: status,
	}
	
	return pod, nil
}