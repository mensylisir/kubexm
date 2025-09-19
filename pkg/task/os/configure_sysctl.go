package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type ConfigureSysctlTask struct {
	task.Base
}

func NewConfigureSysctlTask() task.Task {
	return &ConfigureSysctlTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ConfigureSysctl",
				Description: "Configure kernel parameters (sysctl) on all nodes",
			},
		},
	}
}

func (t *ConfigureSysctlTask) Name() string {
	return t.Meta.Name
}

func (t *ConfigureSysctlTask) Description() string {
	return t.Meta.Description
}

func (t *ConfigureSysctlTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *ConfigureSysctlTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	configureSysctlStep := osstep.NewConfigureSysctlStepBuilder(*runtimeCtx, "ConfigureSysctl").Build()

	node := &plan.ExecutionNode{
		Name:  "ConfigureSysctl",
		Step:  configureSysctlStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
