package engine

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	// "github.com/mensylisir/kubexm/pkg/runner" // Removed as unused
	// "github.com/mensylisir/kubexm/pkg/cache" // No longer needed here, StepContext defines its cache needs
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For Cluster type
	// "time" // No longer needed here for time.Duration directly
	"github.com/mensylisir/kubexm/pkg/step" // For step.StepContext
)

// EngineExecuteContext is passed to the Engine's Execute method.
// It provides the engine with necessary dependencies and configurations.
type EngineExecuteContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	ForHost(host connector.Host) step.StepContext // Returns a step.StepContext for a specific host
}

// Engine is responsible for executing a given ExecutionGraph.
// Its implementation must be a DAG scheduler.
type Engine interface {
	Execute(ctx EngineExecuteContext, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
