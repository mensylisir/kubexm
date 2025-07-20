package pipeline

import (
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type Pipeline interface {
	Name() string
	Description() string
	Modules() []module.Module
	Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error)
	Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
	GetBase() *Base
}
