package handler

import (
	"net/http"

	"github.com/gin-gonic/gin" // Assuming Gin framework
	"github.com/mensylisir/kubexm/rest/app" // To use app.ClusterService and request/response structs
	"github.com/mensylisir/kubexm/pkg/logger" // For logging within handler
)

// ClusterHandler handles HTTP requests related to clusters.
type ClusterHandler struct {
	service *app.ClusterService
	log     *logger.Logger
}

// NewClusterHandler creates a new ClusterHandler.
func NewClusterHandler(service *app.ClusterService, log *logger.Logger) *ClusterHandler {
	return &ClusterHandler{
		service: service,
		log:     log.With("component", "cluster-handler"),
	}
}

// CreateCluster godoc
// @Summary Create a new Kubernetes cluster
// @Description Initiates the creation of a new Kubernetes cluster based on the provided YAML configuration.
// @Tags clusters
// @Accept json
// @Produce json
// @Param clusterCreationRequest body app.CreateClusterRequest true "Cluster Creation Request"
// @Success 202 {object} app.CreateClusterResponse "Cluster creation initiated"
// @Failure 400 {object} ErrorResponse "Invalid request payload"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/clusters [post]
func (h *ClusterHandler) CreateCluster(c *gin.Context) {
	h.log.Info("Received request to create cluster.")
	var req app.CreateClusterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Error(err, "Failed to bind JSON request for create cluster")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request payload: " + err.Error()})
		return
	}

	if req.ClusterConfigYAML == "" {
		h.log.Error(nil, "ClusterConfigYAML is empty in create cluster request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "clusterConfigYAML cannot be empty"})
		return
	}

	// Call the application service to initiate cluster creation
	// For a true async API, this would immediately return a task ID.
	// Our current service.CreateClusterAsync launches a goroutine.
	resp, err := h.service.CreateClusterAsync(req)
	if err != nil {
		h.log.Error(err, "Cluster creation initiation failed")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to initiate cluster creation: " + err.Error()})
		return
	}

	h.log.Info("Cluster creation initiated successfully.", "taskID", resp.TaskID)
	c.JSON(http.StatusAccepted, resp) // 202 Accepted for async operations
}

// GetClusterTaskStatus godoc
// @Summary Get the status of a cluster creation task
// @Description Retrieves the status and result of a specific cluster creation task.
// @Tags clusters
// @Produce json
// @Param taskId path string true "Task ID"
// @Success 200 {object} app.GetTaskStatusResponse "Task status"
// @Failure 404 {object} ErrorResponse "Task not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/tasks/{taskId} [get]
func (h *ClusterHandler) GetClusterTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	h.log.Info("Received request to get task status.", "taskID", taskID)

	if taskID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Task ID cannot be empty"})
		return
	}

	statusResp, err := h.service.GetTaskStatus(taskID)
	if err != nil {
		// Differentiate between "not found" and other errors if service returns specific error types
		h.log.Warn("Failed to get task status.", "taskID", taskID, "error", err)
		// Assuming error means not found for simplicity here.
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	h.log.Info("Successfully retrieved task status.", "taskID", taskID, "status", statusResp.Status)
	c.JSON(http.StatusOK, statusResp)
}


// ErrorResponse is a generic error JSON response.
type ErrorResponse struct {
	Error string `json:"error"`
}
```
