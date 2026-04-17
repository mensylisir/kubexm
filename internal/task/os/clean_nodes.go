package os

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	osstep "github.com/mensylisir/kubexm/internal/step/os"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanOSNodesTask struct {
	task.Base
}

func NewCleanOSNodesTask() task.Task {
	return &CleanOSNodesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanOSNodes",
				Description: "Clean up OS settings on all nodes (remove hosts entries, re-enable services, etc.)",
			},
		},
	}
}

func (t *CleanOSNodesTask) Name() string {
	return t.Meta.Name
}

func (t *CleanOSNodesTask) Description() string {
	return t.Meta.Description
}

func (t *CleanOSNodesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanOSNodesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	enableSwap, err := osstep.NewEnableSwapStepBuilder(runtimeCtx, "EnableSwap").Build()
	if err != nil {
		return nil, err
	}
	enableSelinux, err := osstep.NewEnableSelinuxStepBuilder(runtimeCtx, "EnableSelinux").Build()
	if err != nil {
		return nil, err
	}
	enableFirewall, err := osstep.NewEnableFirewallStepBuilder(runtimeCtx, "EnableFirewall").Build()
	if err != nil {
		return nil, err
	}
	removeEtcHosts, err := osstep.NewRemoveEtcHostsStepBuilder(runtimeCtx, "RemoveEtcHosts").Build()
	if err != nil {
		return nil, err
	}
	removeKernelModules, err := osstep.NewRemoveKernelModulesStepBuilder(runtimeCtx, "RemoveKernelModules").Build()
	if err != nil {
		return nil, err
	}
	removeSysctl, err := osstep.NewRemoveSysctlStepBuilder(runtimeCtx, "RemoveSysctl").Build()
	if err != nil {
		return nil, err
	}
	removeSecurityLimits, err := osstep.NewRemoveSecurityLimitsStepBuilder(runtimeCtx, "RemoveSecurityLimits").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "EnableSwap", Step: enableSwap, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableSelinux", Step: enableSelinux, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableFirewall", Step: enableFirewall, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveEtcHosts", Step: removeEtcHosts, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveKernelModules", Step: removeKernelModules, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveSysctl", Step: removeSysctl, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveSecurityLimits", Step: removeSecurityLimits, Hosts: allHosts})

	// Set up dependencies for safe cleanup order:
	// 1. Remove hosts entries first (most important cluster cleanup)
	// 2. Re-enable services (swap, selinux, firewall) can run in parallel
	// 3. Remove kernel modules, sysctl, and security limits can run in parallel after services
	fragment.AddDependency("RemoveEtcHosts", "EnableSwap")
	fragment.AddDependency("RemoveEtcHosts", "EnableSelinux")
	fragment.AddDependency("RemoveEtcHosts", "EnableFirewall")
	fragment.AddDependency("EnableSwap", "RemoveKernelModules")
	fragment.AddDependency("EnableSelinux", "RemoveSysctl")
	fragment.AddDependency("EnableFirewall", "RemoveSecurityLimits")
	return fragment, nil
}
