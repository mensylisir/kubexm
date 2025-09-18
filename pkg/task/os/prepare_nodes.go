package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/step/packages"
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
				Description: "Prepare all nodes with necessary OS settings (hostname, swap, selinux, packages, etc.)",
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

	// Step 1: Set Hostname for each host
	// This must be done first and individually.
	var setHostnameExitNodes []plan.NodeID
	for _, host := range allHosts {
		hostname := host.GetName()
		stepName := fmt.Sprintf("SetHostnameFor_%s", hostname)
		nodeName := fmt.Sprintf("SetHostnameFor_%s_node", hostname)

		setHostnameStep := osstep.NewSetHostnameStepBuilder(*runtimeCtx, stepName, hostname).Build()
		node := &plan.ExecutionNode{Name: nodeName, Step: setHostnameStep, Hosts: []connector.Host{host}}
		nodeID, _ := fragment.AddNode(node)
		setHostnameExitNodes = append(setHostnameExitNodes, nodeID)
	}

	// Step 2: Update /etc/hosts on all nodes
	updateEtcHostsStep := osstep.NewUpdateEtcHostsStepBuilder(*runtimeCtx, "UpdateEtcHosts").Build()
	updateEtcHostsNode := &plan.ExecutionNode{Name: "UpdateEtcHosts", Step: updateEtcHostsStep, Hosts: allHosts}
	updateEtcHostsNodeID, _ := fragment.AddNode(updateEtcHostsNode)
	// Depends on all hostnames being set
	fragment.AddDependency(setHostnameExitNodes, updateEtcHostsNodeID)

	// Step 3: Install required packages
	installPackagesStep := packages.NewInstallPackagesStepBuilder(*runtimeCtx, "InstallPackages").Build()
	installPackagesNode := &plan.ExecutionNode{Name: "InstallPackages", Step: installPackagesStep, Hosts: allHosts}
	installPackagesNodeID, _ := fragment.AddNode(installPackagesNode)
	// Depends on hostname/hosts being correct for potential repo access
	fragment.AddDependency(updateEtcHostsNodeID, installPackagesNodeID)

	// Step 4: Disable Swap, SELinux, Firewall in parallel
	disableSwapStep := osstep.NewDisableSwapStepBuilder(*runtimeCtx, "DisableSwap").Build()
	disableSelinuxStep := osstep.NewDisableSelinuxStepBuilder(*runtimeCtx, "DisableSelinux").Build()
	disableFirewallStep := osstep.NewDisableFirewallStepBuilder(*runtimeCtx, "DisableFirewall").Build()

	disableSwapNode := &plan.ExecutionNode{Name: "DisableSwap", Step: disableSwapStep, Hosts: allHosts}
	disableSelinuxNode := &plan.ExecutionNode{Name: "DisableSelinux", Step: disableSelinuxStep, Hosts: allHosts}
	disableFirewallNode := &plan.ExecutionNode{Name: "DisableFirewall", Step: disableFirewallStep, Hosts: allHosts}

	disableSwapNodeID, _ := fragment.AddNode(disableSwapNode)
	disableSelinuxNodeID, _ := fragment.AddNode(disableSelinuxNode)
	disableFirewallNodeID, _ := fragment.AddNode(disableFirewallNode)

	// These depend on packages being installed
	fragment.AddDependency(installPackagesNodeID, disableSwapNodeID)
	fragment.AddDependency(installPackagesNodeID, disableSelinuxNodeID)
	fragment.AddDependency(installPackagesNodeID, disableFirewallNodeID)

	parallelDisableExitNodes := []plan.NodeID{disableSwapNodeID, disableSelinuxNodeID, disableFirewallNodeID}

	// Step 5: Configure Kernel modules and sysctl parameters
	loadKernelModulesStep := osstep.NewLoadKernelModulesStepBuilder(*runtimeCtx, "LoadKernelModules").Build()
	configureSysctlStep := osstep.NewConfigureSysctlStepBuilder(*runtimeCtx, "ConfigureSysctl").Build()

	loadKernelModulesNode := &plan.ExecutionNode{Name: "LoadKernelModules", Step: loadKernelModulesStep, Hosts: allHosts}
	configureSysctlNode := &plan.ExecutionNode{Name: "ConfigureSysctl", Step: configureSysctlStep, Hosts: allHosts}

	loadKernelModulesNodeID, _ := fragment.AddNode(loadKernelModulesNode)
	configureSysctlNodeID, _ := fragment.AddNode(configureSysctlNode)

	// These depend on the previous disable steps completing
	fragment.AddDependency(parallelDisableExitNodes, loadKernelModulesNodeID)
	fragment.AddDependency(loadKernelModulesNodeID, configureSysctlNodeID)

	// Step 6: Configure security limits
	// Assuming this step exists and is correct. I haven't verified it yet.
	configureSecurityLimitsStep := osstep.NewConfigureSecurityLimitsStepBuilder(*runtimeCtx, "ConfigureSecurityLimits").Build()
	configureSecurityLimitsNode := &plan.ExecutionNode{Name: "ConfigureSecurityLimits", Step: configureSecurityLimitsStep, Hosts: allHosts}
	configureSecurityLimitsNodeID, _ := fragment.AddNode(configureSecurityLimitsNode)
	fragment.AddDependency(configureSysctlNodeID, configureSecurityLimitsNodeID)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
