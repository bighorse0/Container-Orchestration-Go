package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"mini-k8s-orchestration/internal/storage"
)

// Server represents the HTTP API server
type Server struct {
	router     *gin.Engine
	repository storage.Repository
	port       int
}

// Router returns the gin router for testing
func (s *Server) Router() *gin.Engine {
	return s.router
}

// NewServer creates a new API server
func NewServer(repository storage.Repository, port int) *Server {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)
	
	router := gin.New()
	
	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(requestValidationMiddleware())
	
	server := &Server{
		router:     router,
		repository: repository,
		port:       port,
	}
	
	// Setup routes
	server.setupRoutes()
	
	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting API server on %s\n", addr)
	return s.router.Run(addr)
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthHandler)
	
	// API version group
	v1 := s.router.Group("/api/v1")
	
	// Pod endpoints
	pods := v1.Group("/pods")
	{
		pods.POST("", s.createPod)
		pods.GET("/:name", s.getPod)
		pods.PUT("/:name", s.updatePod)
		pods.DELETE("/:name", s.deletePod)
		pods.GET("", s.listPods)
	}
	
	// Namespaced pod endpoints
	namespacedPods := v1.Group("/namespaces/:namespace/pods")
	{
		namespacedPods.POST("", s.createNamespacedPod)
		namespacedPods.GET("/:name", s.getNamespacedPod)
		namespacedPods.PUT("/:name", s.updateNamespacedPod)
		namespacedPods.DELETE("/:name", s.deleteNamespacedPod)
		namespacedPods.GET("", s.listNamespacedPods)
	}
	
	// Service endpoints
	services := v1.Group("/services")
	{
		services.POST("", s.createService)
		services.GET("/:name", s.getService)
		services.PUT("/:name", s.updateService)
		services.DELETE("/:name", s.deleteService)
		services.GET("", s.listServices)
	}
	
	// Namespaced service endpoints
	namespacedServices := v1.Group("/namespaces/:namespace/services")
	{
		namespacedServices.POST("", s.createNamespacedService)
		namespacedServices.GET("/:name", s.getNamespacedService)
		namespacedServices.PUT("/:name", s.updateNamespacedService)
		namespacedServices.DELETE("/:name", s.deleteNamespacedService)
		namespacedServices.GET("", s.listNamespacedServices)
	}
	
	// Deployment endpoints
	deployments := v1.Group("/deployments")
	{
		deployments.POST("", s.createDeployment)
		deployments.GET("/:name", s.getDeployment)
		deployments.PUT("/:name", s.updateDeployment)
		deployments.DELETE("/:name", s.deleteDeployment)
		deployments.GET("", s.listDeployments)
	}
	
	// Namespaced deployment endpoints
	namespacedDeployments := v1.Group("/namespaces/:namespace/deployments")
	{
		namespacedDeployments.POST("", s.createNamespacedDeployment)
		namespacedDeployments.GET("/:name", s.getNamespacedDeployment)
		namespacedDeployments.PUT("/:name", s.updateNamespacedDeployment)
		namespacedDeployments.DELETE("/:name", s.deleteNamespacedDeployment)
		namespacedDeployments.GET("", s.listNamespacedDeployments)
	}
	
	// Node endpoints (cluster-scoped, no namespace)
	nodes := v1.Group("/nodes")
	{
		nodes.POST("", s.createNode)
		nodes.GET("/:name", s.getNode)
		nodes.PUT("/:name", s.updateNode)
		nodes.DELETE("/:name", s.deleteNode)
		nodes.GET("", s.listNodes)
		nodes.POST("/:name/heartbeat", s.updateNodeHeartbeat)
	}
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// requestValidationMiddleware validates common request parameters
func requestValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add request ID for tracing
		c.Header("X-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))
		c.Next()
	}
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "v1.0.0",
	})
}