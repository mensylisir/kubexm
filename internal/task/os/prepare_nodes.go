package os

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	osstep "github.com/mensylisir/kubexm/internal/step/os"
	"github.com/mensylisir/kubexm/internal/task"
)

type PrepareOSNodesTask struct {
	task.Base
}

func NewPrepareOSNodesTask() task.Task {
	return &PrepareOSNodesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PrepareOSNodes",
				Description: "Prepare all nodes with necessary OS settings (disable swap, selinux, configure sysctl, etc.)",
			},
		},
	}
}

func (t *PrepareOSNodesTask) Name() string {
	return t.Meta.Name
}

func (t *PrepareOSNodesTask) Description() string {
	return t.Meta.Description
}

func (t *PrepareOSNodesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *PrepareOSNodesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	disableSwap, err := osstep.NewDisableSwapStepBuilder(runtimeCtx, "DisableSwap").Build()
	if err != nil {
		return nil, err
	}
	disableSelinux, err := osstep.NewDisableSelinuxStepBuilder(runtimeCtx, "DisableSelinux").Build()
	if err != nil {
		return nil, err
	}
	disableFirewall, err := osstep.NewDisableFirewallStepBuilder(runtimeCtx, "DisableFirewall").Build()
	if err != nil {
		return nil, err
	}
	updateEtcHosts, err := osstep.NewUpdateEtcHostsStepBuilder(runtimeCtx, "UpdateEtcHosts").Build()
	if err != nil {
		return nil, err
	}
	loadKernelModules, err := osstep.NewLoadKernelModulesStepBuilder(runtimeCtx, "LoadKernelModules").Build()
	if err != nil {
		return nil, err
	}
	configureSysctl, err := osstep.NewConfigureSysctlStepBuilder(runtimeCtx, "ConfigureSysctl").Build()
	if err != nil {
		return nil, err
	}
	configureSecurityLimits, err := osstep.NewConfigureSecurityLimitsStepBuilder(runtimeCtx, "ConfigureSecurityLimits").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "DisableSwap", Step: disableSwap, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableSelinux", Step: disableSelinux, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableFirewall", Step: disableFirewall, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "UpdateEtcHosts", Step: updateEtcHosts, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "LoadKernelModules", Step: loadKernelModules, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureSysctl", Step: configureSysctl, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureSecurityLimits", Step: configureSecurityLimits, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
