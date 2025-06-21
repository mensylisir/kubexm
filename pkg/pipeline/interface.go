package pipeline

import (
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"    // Uses plan.ExecutionGraph and plan.GraphExecutionResult
	"github.com/mensylisir/kubexm/pkg/runtime"
)

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
	Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error)

	// Run now takes the full runtime Context and a dryRun flag.
	// It will call its Plan() method to get the ExecutionGraph,
	// then pass this graph to the Engine for execution.
	// It returns a GraphExecutionResult.
	Run(ctx *runtime.Context, dryRun bool) (*plan.GraphExecutionResult, error)
}
