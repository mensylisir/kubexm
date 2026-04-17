package pipeline

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// SafePlan wraps a Plan function with panic recovery.
// This prevents a module panic from crashing the entire pipeline planning phase.
func SafePlan(
	ctx runtime.PipelineContext,
	pipelineName string,
	planFn func() (*plan.ExecutionGraph, error),
) (*plan.ExecutionGraph, error) {
	var result *plan.ExecutionGraph
	var err error

	logger := ctx.GetLogger()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC during Plan phase in pipeline %s: %v", pipelineName, r)
			logger.Error(err, "Pipeline panicked during Plan", "panic", r)
		}
	}()

	result, err = planFn()
	return result, err
}

// SafeModulePlan wraps a module Plan call with panic recovery.
func SafeModulePlan(
	moduleCtx runtime.ModuleContext,
	pipelineName string,
	mod module.Module,
) (*plan.ExecutionFragment, error) {
	var result *plan.ExecutionFragment
	var err error

	logger := moduleCtx.GetLogger()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC while planning module %s in pipeline %s: %v", mod.Name(), pipelineName, r)
			logger.Error(err, "Module panicked during Plan", "module", mod.Name(), "panic", r)
		}
	}()

	result, err = mod.Plan(moduleCtx)
	return result, err
}
