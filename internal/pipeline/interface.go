package pipeline

import (
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

type Pipeline interface {
	Name() string
	Description() string
	Modules() []module.Module
	Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error)
	Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
	GetBase() *Base
}
