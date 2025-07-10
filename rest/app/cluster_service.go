package app

import (
	"context"
	"fmt"
	"path/filepath" // For potential temporary file handling if YAML is string
	"os" // For potential temporary file handling

	"gopkg.in/yaml.v3"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	kubexmcluster "github.com/mensylisir/kubexm/pkg/pipeline/cluster"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/runner" // Added for runner.New()
	"github.com/google/uuid" // For generating task IDs
	"sync"                   // For sync.Mutex
)

// ClusterService provides operations related to cluster management.
type ClusterService struct {
	// Potentially shared services if the API server is long-running
	// For now, instantiate them per request or assume they are globally available if simple.
	// Example:
	// connPool *connector.ConnectionPool
	// factory  connector.Factory
	// runnerSvc runner.Runner
	// engineSvc engine.Engine
	Log *logger.Logger
}

// NewClusterService creates a new ClusterService.
func NewClusterService(log *logger.Logger) *ClusterService {
	return &ClusterService{
		Log: log,
	}
}

// CreateClusterRequest defines the request payload for creating a cluster.
// It can accept raw YAML content or a path to a configuration file.
// For a REST API, raw YAML content is more common.
type CreateClusterRequest struct {
	ClusterConfigYAML string `json:"clusterConfigYAML"`
	// AssumeYes for non-interactive mode within the pipeline (e.g. for preflight confirmations)
	AssumeYes         bool   `json:"assumeYes"`
	DryRun            bool   `json:"dryRun"`
}

// CreateClusterResponse defines the response for a cluster creation request.
type CreateClusterResponse struct {
	TaskID  string `json:"taskID"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TaskResultStore (In-memory for simplicity, should be persistent in production)
var taskResults = make(map[string]*plan.GraphExecutionResult)
var taskStatus = make(map[string]string) // "pending", "running", "completed", "failed"
var taskErrors = make(map[string]string)
var mu sync.Mutex // For thread-safe access to taskResults, taskStatus, taskErrors

// CreateClusterAsync handles the asynchronous creation of a cluster.
// For a true async implementation, this would involve a task queue.
// Here, we simulate it by launching a goroutine.
func (s *ClusterService) CreateClusterAsync(req CreateClusterRequest) (*CreateClusterResponse, error) {
	s.Log.Info("Received asynchronous cluster creation request.")
	taskID := uuid.New().String()

	mu.Lock()
	taskStatus[taskID] = "pending"
	mu.Unlock()

	go func() {
		s.Log.Infof("Starting background task for cluster creation: %s", taskID)

		// For simplicity, services are newed up here. In a real server,
		// some (like pool, factory) might be shared and passed to ClusterService.
		connectorFactory := connector.NewDefaultFactory()
		connectionPool := connector.NewConnectionPool(connector.DefaultPoolConfig())
		defer connectionPool.Shutdown()
		runnerSvc := runner.New()
		engineSvc := engine.NewExecutor()

		// Create a temporary file for the YAML content if needed by config.ParseFromFile
		// Or modify config.ParseFromFile to accept a byte slice.
		// For now, assume config.ParseFromFile takes a filepath.
		tmpFile, err := os.CreateTemp("", "cluster-config-*.yaml")
		if err != nil {
			s.Log.Errorf("Failed to create temp file for task %s: %v", taskID, err)
			mu.Lock()
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = "Failed to create temp config file"
			mu.Unlock()
			return
		}
		defer os.Remove(tmpFile.Name()) // Clean up

		if _, err := tmpFile.Write([]byte(req.ClusterConfigYAML)); err != nil {
			s.Log.Errorf("Failed to write YAML to temp file for task %s: %v", taskID, err)
			mu.Lock()
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = "Failed to write YAML to temp file"
			mu.Unlock()
			return
		}
		if err := tmpFile.Close(); err != nil {
			s.Log.Errorf("Failed to close temp file for task %s: %v", taskID, err)
			mu.Lock()
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = "Failed to close temp file"
			mu.Unlock()
			return
		}

		clusterConfig, err := config.ParseFromFile(tmpFile.Name())
		if err != nil {
			s.Log.Errorf("Failed to parse cluster configuration for task %s: %v", taskID, err)
			mu.Lock()
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = fmt.Sprintf("Failed to parse cluster configuration: %s", err.Error())
			mu.Unlock()
			return
		}

		// Apply overrides from request (e.g., SkipPreflight if it were part of CreateClusterRequest)
		// For now, only AssumeYes and DryRun are passed to pipeline.

		rtBuilder := runtime.NewRuntimeBuilderFromConfig(clusterConfig, runnerSvc, connectionPool, connectorFactory)

		mu.Lock()
		taskStatus[taskID] = "building_runtime"
		mu.Unlock()

		runtimeCtx, cleanupFunc, err := rtBuilder.Build(context.Background(), engineSvc)
		if err != nil {
			s.Log.Errorf("Failed to build runtime for task %s: %v", taskID, err)
			if cleanupFunc != nil { defer cleanupFunc() }
			mu.Lock()
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = fmt.Sprintf("Failed to build runtime: %s", err.Error())
			mu.Unlock()
			return
		}
		if cleanupFunc != nil { defer cleanupFunc() }

		pipeline := kubexmcluster.NewCreateClusterPipeline(req.AssumeYes)

		mu.Lock()
		taskStatus[taskID] = "planning_pipeline"
		mu.Unlock()

		graph, err := pipeline.Plan(runtimeCtx) // runtimeCtx implements pipeline.PipelineContext
		if err != nil {
			s.Log.Errorf("Pipeline planning failed for task %s: %v", taskID, err)
			mu.Lock()
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = fmt.Sprintf("Pipeline planning failed: %s", err.Error())
			mu.Unlock()
			return
		}

		mu.Lock()
		taskStatus[taskID] = "running_pipeline"
		mu.Unlock()

		result, err := pipeline.Run(runtimeCtx, graph, req.DryRun) // runtimeCtx implements pipeline.PipelineContext

		mu.Lock()
		taskResults[taskID] = result
		if err != nil {
			s.Log.Errorf("Pipeline execution failed for task %s: %v", taskID, err)
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = fmt.Sprintf("Pipeline execution failed: %s", err.Error())
		} else if result != nil && result.Status == plan.StatusFailed {
			s.Log.Errorf("Pipeline execution reported failure for task %s. Status: %s", taskID, result.Status)
			taskStatus[taskID] = "failed"
			taskErrors[taskID] = fmt.Sprintf("Pipeline failed with status: %s. Message: %s", result.Status, result.ErrorMessage)
		} else {
			s.Log.Infof("Pipeline execution completed for task %s. Status: %s", taskID, result.Status)
			taskStatus[taskID] = "completed"
		}
		mu.Unlock()
	}()

	return &CreateClusterResponse{TaskID: taskID, Status: "pending", Message: "Cluster creation initiated."}, nil
}

// GetTaskStatusResponse defines the response for getting task status.
type GetTaskStatusResponse struct {
	TaskID  string `json:"taskID"`
	Status  string `json:"status"` // e.g., "pending", "running", "completed", "failed"
	Message string `json:"message,omitempty"` // Error message if failed
	Result  *plan.GraphExecutionResult `json:"result,omitempty"` // Full result if completed/failed
}


// GetTaskStatus retrieves the status and result of a previously submitted task.
func (s *ClusterService) GetTaskStatus(taskID string) (*GetTaskStatusResponse, error) {
	mu.Lock()
	defer mu.Unlock()

	status, found := taskStatus[taskID]
	if !found {
		return nil, fmt.Errorf("task with ID '%s' not found", taskID)
	}

	resp := &GetTaskStatusResponse{
		TaskID: taskID,
		Status: status,
	}

	if errMsg, foundErr := taskErrors[taskID]; foundErr {
		resp.Message = errMsg
	}
	if result, foundRes := taskResults[taskID]; foundRes {
		resp.Result = result
	}

	// Clean up very old tasks (simple example, not robust for production)
	// if status == "completed" || status == "failed" {
	// delete(taskStatus, taskID)
	// delete(taskErrors, taskID)
	// delete(taskResults, taskID)
	// }

	return resp, nil
}
```
