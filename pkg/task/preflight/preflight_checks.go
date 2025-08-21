package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type PreflightChecksTask struct {
	task.Base
}

func NewPreflightChecksTask() task.Task {
	return &PreflightChecksTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PreflightChecks",
				Description: "Run preflight checks (CPU, memory, etc.) on all nodes before installation",
			},
		},
	}
}

func (t *PreflightChecksTask) Name() string {
	return t.Meta.Name
}

func (t *PreflightChecksTask) Description() string {
	return t.Meta.Description
}

func (t *PreflightChecksTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Preflight != nil {
		for _, check := range ctx.GetClusterConfig().Spec.Preflight.SkipChecks {
			if check == "all" {
				ctx.GetLogger().Info("Skipping all preflight checks because 'all' is in skipChecks list.")
				return false, nil
			}
		}
	}
	return true, nil
}

func (t *PreflightChecksTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	checkCPU := preflightstep.NewCheckCPUStepBuilder(*runtimeCtx, "CheckMinCPUCores").Build()
	checkMemory := preflightstep.NewCheckMemoryStepBuilder(*runtimeCtx, "CheckMinMemory").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CheckMinCPUCores", Step: checkCPU, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckMinMemory", Step: checkMemory, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
