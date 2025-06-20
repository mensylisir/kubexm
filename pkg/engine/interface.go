package engine

import (
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
)

// Engine is responsible for executing an ExecutionPlan.
type Engine interface {
	Execute(ctx *runtime.Context, p *plan.ExecutionPlan, dryRun bool) (*plan.ExecutionResult, error)
}
