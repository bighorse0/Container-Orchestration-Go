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

// createDeployment handles POST /api/v1/deployments
func (s *Server) createDeployment(c *gin.Context) {
	s.createDeploymentInNamespace(c, "default")
}

// createNamespacedDeployment handles POST /api/v1/namespaces/{namespace}/deployments
func (s *Server) createNamespacedDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.createDeploymentInNamespace(c, namespace)
}

// createDeploymentInNamespace creates a deployment in the specified namespace
func (s *Server) createDeploymentInNamespace(c *gin.Context, namespace string) {
	var deployment types.Deployment
	
	if err := c.ShouldBindJSON(&deployment); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set namespace if not provided
	if deployment.Metadata.Namespace == "" {
		deployment.Metadata.Namespace = namespace
	}
	
	// Validate namespace matches URL parameter
	if deployment.Metadata.Namespace != namespace {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAMESPACE_MISMATCH",
			Message: "Deployment namespace does not match URL namespace",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the deployment
	if err := types.ValidateDeployment(&deployment); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Deployment validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set metadata
	now := time.Now()
	deployment.APIVersion = "apps/v1"
	deployment.Kind = "Deployment"
	deployment.Metadata.UID = uuid.New().String()
	deployment.Metadata.CreatedAt = now
	deployment.Metadata.UpdatedAt = now
	
	// Initialize status
	deployment.Status = types.DeploymentStatus{
		ObservedGeneration:  1,
		Replicas:            0,
		UpdatedReplicas:     0,
		ReadyReplicas:       0,
		AvailableReplicas:   0,
		UnavailableReplicas: deployment.Spec.Replicas,
		Conditions: []types.DeploymentCondition{
			{
				Type:               "Progressing",
				Status:             "True",
				LastUpdateTime:     now,
				LastTransitionTime: now,
				Reason:             "NewReplicaSetCreated",
				Message:            "Created new replica set",
			},
		},
	}
	
	// Convert to storage resource
	specJSON, err := json.Marshal(deployment.Spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize deployment spec",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	statusJSON, err := json.Marshal(deployment.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize deployment status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	resource := storage.Resource{
		ID:        deployment.Metadata.UID,
		Kind:      "Deployment",
		Namespace: deployment.Metadata.Namespace,
		Name:      deployment.Metadata.Name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}
	
	// Save to database
	if err := s.repository.CreateResource(resource); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "RESOURCE_EXISTS",
			Message: "Deployment already exists",
			Code:    http.StatusConflict,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusCreated, deployment)
}

// getDeployment handles GET /api/v1/deployments/{name}
func (s *Server) getDeployment(c *gin.Context) {
	s.getDeploymentFromNamespace(c, "default")
}

// getNamespacedDeployment handles GET /api/v1/namespaces/{namespace}/deployments/{name}
func (s *Server) getNamespacedDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.getDeploymentFromNamespace(c, namespace)
}

// getDeploymentFromNamespace gets a deployment from the specified namespace
func (s *Server) getDeploymentFromNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Deployment name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Get from database
	resource, err := s.repository.GetResource("Deployment", namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Deployment not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	// Convert to Deployment
	deployment, err := s.resourceToDeployment(resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DESERIALIZATION_ERROR",
			Message: "Failed to deserialize deployment",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, deployment)
}

// updateDeployment handles PUT /api/v1/deployments/{name}
func (s *Server) updateDeployment(c *gin.Context) {
	s.updateDeploymentInNamespace(c, "default")
}

// updateNamespacedDeployment handles PUT /api/v1/namespaces/{namespace}/deployments/{name}
func (s *Server) updateNamespacedDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.updateDeploymentInNamespace(c, namespace)
}

// updateDeploymentInNamespace updates a deployment in the specified namespace
func (s *Server) updateDeploymentInNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Deployment name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	var deployment types.Deployment
	if err := c.ShouldBindJSON(&deployment); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Validate name matches URL parameter
	if deployment.Metadata.Name != name {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAME_MISMATCH",
			Message: "Deployment name does not match URL parameter",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate namespace
	if deployment.Metadata.Namespace != namespace {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAMESPACE_MISMATCH",
			Message: "Deployment namespace does not match URL namespace",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the deployment
	if err := types.ValidateDeployment(&deployment); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Deployment validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Update timestamp
	deployment.Metadata.UpdatedAt = time.Now()
	
	// Convert to storage resource
	specJSON, err := json.Marshal(deployment.Spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize deployment spec",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	statusJSON, err := json.Marshal(deployment.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize deployment status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	resource := storage.Resource{
		Kind:      "Deployment",
		Namespace: namespace,
		Name:      name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}
	
	// Update in database
	if err := s.repository.UpdateResource(resource); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Deployment not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, deployment)
}

// deleteDeployment handles DELETE /api/v1/deployments/{name}
func (s *Server) deleteDeployment(c *gin.Context) {
	s.deleteDeploymentFromNamespace(c, "default")
}

// deleteNamespacedDeployment handles DELETE /api/v1/namespaces/{namespace}/deployments/{name}
func (s *Server) deleteNamespacedDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.deleteDeploymentFromNamespace(c, namespace)
}

// deleteDeploymentFromNamespace deletes a deployment from the specified namespace
func (s *Server) deleteDeploymentFromNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Deployment name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Delete from database
	if err := s.repository.DeleteResource("Deployment", namespace, name); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Deployment not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Deployment deleted successfully",
	})
}

// listDeployments handles GET /api/v1/deployments
func (s *Server) listDeployments(c *gin.Context) {
	s.listDeploymentsInNamespace(c, "")
}

// listNamespacedDeployments handles GET /api/v1/namespaces/{namespace}/deployments
func (s *Server) listNamespacedDeployments(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.listDeploymentsInNamespace(c, namespace)
}

// listDeploymentsInNamespace lists deployments in the specified namespace
func (s *Server) listDeploymentsInNamespace(c *gin.Context, namespace string) {
	// Get from database
	resources, err := s.repository.ListResources("Deployment", namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to list deployments",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	// Convert to deployments
	var deployments []types.Deployment
	for _, resource := range resources {
		deployment, err := s.resourceToDeployment(resource)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "DESERIALIZATION_ERROR",
				Message: "Failed to deserialize deployment",
				Code:    http.StatusInternalServerError,
				Details: map[string]string{"error": err.Error()},
			})
			return
		}
		deployments = append(deployments, *deployment)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"apiVersion": "apps/v1",
		"kind":       "DeploymentList",
		"items":      deployments,
	})
}

// resourceToDeployment converts a storage resource to a Deployment
func (s *Server) resourceToDeployment(resource storage.Resource) (*types.Deployment, error) {
	var spec types.DeploymentSpec
	if err := json.Unmarshal([]byte(resource.Spec), &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment spec: %w", err)
	}
	
	var status types.DeploymentStatus
	if resource.Status != "" {
		if err := json.Unmarshal([]byte(resource.Status), &status); err != nil {
			return nil, fmt.Errorf("failed to unmarshal deployment status: %w", err)
		}
	}
	
	deployment := &types.Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
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
	
	return deployment, nil
}