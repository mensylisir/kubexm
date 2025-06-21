package engine

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runner"
	// No longer importing full runtime
	"github.com/mensylisir/kubexm/pkg/runtime" // Needed for runtime.StepContext
)

// EngineExecuteContext defines the methods from runtime.Context that engine's Execute needs.
type EngineExecuteContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	// GetRunner() runner.Runner // Steps get runner from StepContext
	ForHost(host connector.Host) runtime.StepContext // Returns StepContext for a specific host
	// Add any other methods from runtime.Context that Execute directly uses (not via StepContext)
}

// Engine is responsible for executing a given ExecutionGraph.
// Its implementation must be a DAG scheduler.
type Engine interface {
	Execute(ctx EngineExecuteContext, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
