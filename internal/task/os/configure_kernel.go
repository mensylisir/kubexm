package os

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	osstep "github.com/mensylisir/kubexm/internal/step/os"
	"github.com/mensylisir/kubexm/internal/task"
)

type ConfigureKernelTask struct {
	task.Base
}

func NewConfigureKernelTask() task.Task {
	return &ConfigureKernelTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ConfigureKernel",
				Description: "Load kernel modules and configure sysctl parameters on all nodes",
			},
		},
	}
}

func (t *ConfigureKernelTask) Name() string {
	return t.Meta.Name
}

func (t *ConfigureKernelTask) Description() string {
	return t.Meta.Description
}

func (t *ConfigureKernelTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *ConfigureKernelTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	loadModulesStep, err := osstep.NewLoadKernelModulesStepBuilder(runtimeCtx, "LoadKernelModules").Build()
	if err != nil {
		return nil, err
	}
	configureSysctlStep, err := osstep.NewConfigureSysctlStepBuilder(runtimeCtx, "ConfigureSysctl").Build()
	if err != nil {
		return nil, err
	}

	loadModulesNode := &plan.ExecutionNode{Name: "LoadKernelModules", Step: loadModulesStep, Hosts: allHosts}
	configureSysctlNode := &plan.ExecutionNode{Name: "ConfigureSysctl", Step: configureSysctlStep, Hosts: allHosts}

	loadModulesNodeID, _ := fragment.AddNode(loadModulesNode)
	configureSysctlNodeID, _ := fragment.AddNode(configureSysctlNode)

	// sysctl depends on modules being loaded
	fragment.AddDependency(loadModulesNodeID, configureSysctlNodeID)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
