package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"mini-k8s-orchestration/pkg/types"
)

// createNode handles POST /api/v1/nodes
func (s *Server) createNode(c *gin.Context) {
	var node types.Node
	
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Validate the node
	if err := types.ValidateNode(&node); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Node validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Set metadata
	now := time.Now()
	node.APIVersion = "v1"
	node.Kind = "Node"
	node.Metadata.UID = uuid.New().String()
	node.Metadata.CreatedAt = now
	node.Metadata.UpdatedAt = now
	
	// Initialize status if not provided
	if len(node.Status.Conditions) == 0 {
		node.Status.Conditions = []types.NodeCondition{
			{
				Type:               "Ready",
				Status:             "Unknown",
				LastHeartbeatTime:  now,
				LastTransitionTime: now,
				Reason:             "NodeStatusNeverUpdated",
				Message:            "Node status has never been updated",
			},
		}
	}
	
	// Save to database using node-specific method
	if err := s.repository.CreateNode(&node); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "RESOURCE_EXISTS",
			Message: "Node already exists",
			Code:    http.StatusConflict,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusCreated, node)
}

// getNode handles GET /api/v1/nodes/{name}
func (s *Server) getNode(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Node name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Get from database using node-specific method
	node, err := s.repository.GetNode(name)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Node not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, node)
}

// updateNode handles PUT /api/v1/nodes/{name}
func (s *Server) updateNode(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Node name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	var node types.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: "Invalid JSON format",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Validate name matches URL parameter
	if node.Metadata.Name != name {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "NAME_MISMATCH",
			Message: "Node name does not match URL parameter",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Validate the node
	if err := types.ValidateNode(&node); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: "Node validation failed",
			Code:    http.StatusBadRequest,
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}
	
	// Update timestamp
	node.Metadata.UpdatedAt = time.Now()
	
	// Update in database using node-specific method
	if err := s.repository.UpdateNode(&node); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Node not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, node)
}

// deleteNode handles DELETE /api/v1/nodes/{name}
func (s *Server) deleteNode(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Node name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Delete from database using node-specific method
	if err := s.repository.DeleteNode(name); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Node not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Node deleted successfully",
	})
}

// listNodes handles GET /api/v1/nodes
func (s *Server) listNodes(c *gin.Context) {
	// Get from database using node-specific method
	nodes, err := s.repository.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to list nodes",
			Code:    http.StatusInternalServerError,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"apiVersion": "v1",
		"kind":       "NodeList",
		"items":      nodes,
	})
}

// updateNodeHeartbeat handles POST /api/v1/nodes/{name}/heartbeat
func (s *Server) updateNodeHeartbeat(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Node name is required",
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Update heartbeat in database
	if err := s.repository.UpdateNodeHeartbeat(name); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "RESOURCE_NOT_FOUND",
			Message: "Node not found",
			Code:    http.StatusNotFound,
			Details: map[string]string{"error": err.Error()},
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":   "Node heartbeat updated successfully",
		"timestamp": time.Now().UTC(),
	})
}