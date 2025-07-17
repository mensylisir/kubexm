package pipeline

import (
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
)

// Pipeline defines the methods that all concrete pipeline types must implement.
// Pipelines are responsible for orchestrating multiple Modules to generate
// the final ExecutionGraph and then running it using the Engine.
type Pipeline interface {
	// Name returns the designated name of the pipeline.
	Name() string

	// Description returns a brief description of the pipeline.
	Description() string

	// Modules returns a list of modules that belong to this pipeline.
	Modules() []module.Module

	// Plan generates the final ExecutionGraph by orchestrating module fragments.
	Plan(ctx PipelineContext) (*plan.ExecutionGraph, error)

	// Run executes the pipeline using the provided ExecutionGraph.
	Run(ctx PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
