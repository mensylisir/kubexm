package pipeline

import (
	"context"                                                // Added
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Added
	"github.com/mensylisir/kubexm/pkg/cache"                 // Added
	"github.com/mensylisir/kubexm/pkg/engine"                // Added for GetEngine method in PipelineContext
	"github.com/mensylisir/kubexm/pkg/logger"                // Added
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan" // Uses plan.ExecutionGraph and plan.GraphExecutionResult
	"github.com/mensylisir/kubexm/pkg/runtime"
	// "github.com/mensylisir/kubexm/pkg/runtime" // REMOVED to break cycle
)

// PipelineContext defines the methods available at the pipeline execution level.
type PipelineContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	GetPipelineCache() cache.PipelineCache
	GetGlobalWorkDir() string
	GetEngine() engine.Engine // Added
}

// Pipeline defines the methods that all concrete pipeline types must implement.
// Pipelines are responsible for orchestrating multiple Modules to generate
// the final ExecutionGraph and then running it using the Engine.
type Pipeline interface {
	// Name returns the designated name of the pipeline.
	Name() string

	// Modules returns a list of modules that belong to this pipeline.
	// Similar to Module.Tasks(), this can be used for introspection or dynamic planning.
	Modules() []module.Module

	// Plan now generates the final, complete ExecutionGraph for the entire pipeline
	// by orchestrating and linking ExecutionFragments from its modules.
	Plan(ctx PipelineContext) (*plan.ExecutionGraph, error) // Changed to local PipelineContext

	// Run now takes the full runtime Context (which implements all necessary sub-contexts
	// like PipelineContext, ModuleContext, TaskContext, StepContext, and EngineExecuteContext)
	// and a dryRun flag.
	// It will call its Plan() method to get the ExecutionGraph,
	// then pass this graph and the full context to the Engine for execution.
	// It returns a GraphExecutionResult.
	Run(ctx *runtime.Context, dryRun bool) (*plan.GraphExecutionResult, error)
}
