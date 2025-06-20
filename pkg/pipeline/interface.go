package pipeline

import (
	"github.com/mensylisir/kubexm/pkg/module"  // Updated import path
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
)

type Pipeline interface {
	Name() string
	Modules() []module.Module // Returns a list of modules that belong to this pipeline
	Plan(ctx runtime.PipelineContext) (*plan.ExecutionPlan, error)
	Run(ctx *runtime.Context, dryRun bool) (*plan.ExecutionResult, error)
}
