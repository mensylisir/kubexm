package server

import (
	"github.com/gin-gonic/gin"
	"github.com/mensylisir/kubexm/rest/app" // For app.ClusterService
	"github.com/mensylisir/kubexm/rest/server/handler"
	"github.com/mensylisir/kubexm/pkg/logger"
)

// SetupRouter configures the HTTP routes for the API server.
func SetupRouter(log *logger.Logger, clusterService *app.ClusterService) *gin.Engine {
	log.Info("Setting up API router...")

	// gin.SetMode(gin.ReleaseMode) // Or gin.DebugMode
	router := gin.Default() // Default includes logger and recovery middleware

	// Health Check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 Group
	apiV1 := router.Group("/api/v1")
	{
		// Cluster routes
		clusterHandler := handler.NewClusterHandler(clusterService, log)
		apiV1.POST("/clusters", clusterHandler.CreateCluster)
		// Task status route
		apiV1.GET("/tasks/:taskId", clusterHandler.GetClusterTaskStatus)

		// TODO: Add other cluster-related routes (GET /clusters, GET /clusters/{id}, DELETE /clusters/{id})
	}

	log.Info("API router setup complete.")
	return router
}
```
