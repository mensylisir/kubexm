package engine

import (
	"github.com/mensylisir/kubexm/pkg/plan"    // Uses plan.ExecutionGraph and plan.GraphExecutionResult
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// Engine is responsible for executing a given ExecutionGraph.
// Its implementation must be a DAG scheduler.
type Engine interface {
	// Execute takes a full runtime Context, an ExecutionGraph, and a dryRun flag.
	// It orchestrates the execution of the graph's nodes according to their dependencies
	// and returns a GraphExecutionResult.
	Execute(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
