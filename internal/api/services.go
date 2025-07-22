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

// createService handles POST /api/v1/services
func (s *Server) createService(c *gin.Context) {
	s.createServiceInNamespace(c, "default")
}

// createNamespacedService handles POST /api/v1/namespaces/{namespace}/services
func (s *Server) createNamespacedService(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.createServiceInNamespace(c, namespace)
}

// createServiceInNamespace creates a service in the specified namespace
func (s *Server) createServiceInNamespace(c *gin.Context, namespace string) {
	var service types.Service
	
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set namespace if not provided
	if service.Metadata.Namespace == "" {
		service.Metadata.Namespace = namespace
	}
	
	// Validate namespace matches URL parameter
	if service.Metadata.Namespace != namespace {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAMESPACE_MISMATCH",
			Message: "Service namespace does not match URL namespace",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the service
	if err := types.ValidateService(&service); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Service validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set metadata
	now := time.Now()
	service.APIVersion = "v1"
	service.Kind = "Service"
	service.Metadata.UID = uuid.New().String()
	service.Metadata.CreatedAt = now
	service.Metadata.UpdatedAt = now
	
	// Initialize status
	service.Status = types.ServiceStatus{
		Endpoints: []types.Endpoint{},
	}
	
	// Convert to storage resource
	specJSON, err := json.Marshal(service.Spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize service spec",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	statusJSON, err := json.Marshal(service.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize service status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	resource := storage.Resource{
		ID:        service.Metadata.UID,
		Kind:      "Service",
		Namespace: service.Metadata.Namespace,
		Name:      service.Metadata.Name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}
	
	// Save to database
	if err := s.repository.CreateResource(resource); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "RESOURCE_EXISTS",
			Message: "Service already exists",
			Code:    http.StatusConflict,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusCreated, service)
}

// getService handles GET /api/v1/services/{name}
func (s *Server) getService(c *gin.Context) {
	s.getServiceFromNamespace(c, "default")
}

// getNamespacedService handles GET /api/v1/namespaces/{namespace}/services/{name}
func (s *Server) getNamespacedService(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.getServiceFromNamespace(c, namespace)
}

// getServiceFromNamespace gets a service from the specified namespace
func (s *Server) getServiceFromNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Service name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Get from database
	resource, err := s.repository.GetResource("Service", namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Service not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	// Convert to Service
	service, err := s.resourceToService(resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DESERIALIZATION_ERROR",
			Message: "Failed to deserialize service",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, service)
}

// updateService handles PUT /api/v1/services/{name}
func (s *Server) updateService(c *gin.Context) {
	s.updateServiceInNamespace(c, "default")
}

// updateNamespacedService handles PUT /api/v1/namespaces/{namespace}/services/{name}
func (s *Server) updateNamespacedService(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.updateServiceInNamespace(c, namespace)
}

// updateServiceInNamespace updates a service in the specified namespace
func (s *Server) updateServiceInNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Service name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	var service types.Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Validate name matches URL parameter
	if service.Metadata.Name != name {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAME_MISMATCH",
			Message: "Service name does not match URL parameter",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate namespace
	if service.Metadata.Namespace != namespace {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAMESPACE_MISMATCH",
			Message: "Service namespace does not match URL namespace",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the service
	if err := types.ValidateService(&service); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Service validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Update timestamp
	service.Metadata.UpdatedAt = time.Now()
	
	// Convert to storage resource
	specJSON, err := json.Marshal(service.Spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize service spec",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	statusJSON, err := json.Marshal(service.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "SERIALIZATION_ERROR",
			Message: "Failed to serialize service status",
			Code:    http.StatusInternalServerError,
		})
		return
	}
	
	resource := storage.Resource{
		Kind:      "Service",
		Namespace: namespace,
		Name:      name,
		Spec:      string(specJSON),
		Status:    string(statusJSON),
	}
	
	// Update in database
	if err := s.repository.UpdateResource(resource); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Service not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, service)
}

// deleteService handles DELETE /api/v1/services/{name}
func (s *Server) deleteService(c *gin.Context) {
	s.deleteServiceFromNamespace(c, "default")
}

// deleteNamespacedService handles DELETE /api/v1/namespaces/{namespace}/services/{name}
func (s *Server) deleteNamespacedService(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.deleteServiceFromNamespace(c, namespace)
}

// deleteServiceFromNamespace deletes a service from the specified namespace
func (s *Server) deleteServiceFromNamespace(c *gin.Context, namespace string) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Service name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Delete from database
	if err := s.repository.DeleteResource("Service", namespace, name); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Service not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Service deleted successfully",
	})
}

// listServices handles GET /api/v1/services
func (s *Server) listServices(c *gin.Context) {
	s.listServicesInNamespace(c, "")
}

// listNamespacedServices handles GET /api/v1/namespaces/{namespace}/services
func (s *Server) listNamespacedServices(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "default"
	}
	s.listServicesInNamespace(c, namespace)
}

// listServicesInNamespace lists services in the specified namespace
func (s *Server) listServicesInNamespace(c *gin.Context, namespace string) {
	// Get from database
	resources, err := s.repository.ListResources("Service", namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to list services",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	// Convert to services
	var services []types.Service
	for _, resource := range resources {
		service, err := s.resourceToService(resource)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "DESERIALIZATION_ERROR",
				Message: "Failed to deserialize service",
				Code:    http.StatusInternalServerError,
				Details: map[string]string{"error": err.Error()},
			})
			return
		}
		services = append(services, *service)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"apiVersion": "v1",
		"kind":       "ServiceList",
		"items":      services,
	})
}

// resourceToService converts a storage resource to a Service
func (s *Server) resourceToService(resource storage.Resource) (*types.Service, error) {
	var spec types.ServiceSpec
	if err := json.Unmarshal([]byte(resource.Spec), &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service spec: %w", err)
	}
	
	var status types.ServiceStatus
	if resource.Status != "" {
		if err := json.Unmarshal([]byte(resource.Status), &status); err != nil {
			return nil, fmt.Errorf("failed to unmarshal service status: %w", err)
		}
	}
	
	service := &types.Service{
		APIVersion: "v1",
		Kind:       "Service",
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
	
	return service, nil
}