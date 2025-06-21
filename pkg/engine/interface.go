package engine

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runner"
	// No longer importing full runtime, runtime.StepContext is replaced by engine.StepContext
	"github.com/mensylisir/kubexm/pkg/cache" // For cache interfaces used in StepContext
)

// StepContext defines the methods available at the step execution level,
// specifically for the engine's perspective. Implementations will be provided by the runtime package.
type StepContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetRunner() runner.Runner
	GetConnectorForHost(host connector.Host) (connector.Connector, error)

	GetHost() connector.Host
	GetCurrentHostFacts() (*runner.Facts, error)
	GetCurrentHostConnector() (connector.Connector, error)

	StepCache() cache.StepCache
	TaskCache() cache.TaskCache
	ModuleCache() cache.ModuleCache
	GetGlobalWorkDir() string

	WithGoContext(goCtx context.Context) StepContext
}

// EngineExecuteContext defines the methods from the broader runtime context
// that the engine's Execute method needs.
type EngineExecuteContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	ForHost(host connector.Host) StepContext // Returns the engine-defined StepContext
	// Add any other methods from the main runtime context that Execute directly uses (not via StepContext)
}

// Engine is responsible for executing a given ExecutionGraph.
// Its implementation must be a DAG scheduler.
type Engine interface {
	Execute(ctx EngineExecuteContext, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
