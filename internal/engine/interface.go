package engine

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

type Engine interface {
	Execute(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
