package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	disableSwap := osstep.NewDisableSwapStepBuilder(*runtimeCtx, "DisableSwap").Build()
	disableSelinux := osstep.NewDisableSelinuxStepBuilder(*runtimeCtx, "DisableSelinux").Build()
	disableFirewall := osstep.NewDisableFirewallStepBuilder(*runtimeCtx, "DisableFirewall").Build()
	updateEtcHosts := osstep.NewUpdateEtcHostsStepBuilder(*runtimeCtx, "UpdateEtcHosts").Build()
	loadKernelModules := osstep.NewLoadKernelModulesStepBuilder(*runtimeCtx, "LoadKernelModules").Build()
	configureSysctl := osstep.NewConfigureSysctlStepBuilder(*runtimeCtx, "ConfigureSysctl").Build()
	configureSecurityLimits := osstep.NewConfigureSecurityLimitsStepBuilder(*runtimeCtx, "ConfigureSecurityLimits").Build()

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
