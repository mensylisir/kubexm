package pipeline

import (
	// Adjust these import paths based on your actual project structure
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type Pipeline interface {
	Name() string
	Modules() []module.Module // Returns a slice of module.Module interfaces
	Plan(ctx runtime.PipelineContext) (*plan.ExecutionPlan, error)
	Run(ctx *runtime.Context, dryRun bool) (*plan.ExecutionResult, error)
}
