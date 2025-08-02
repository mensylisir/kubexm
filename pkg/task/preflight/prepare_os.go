package preflight

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/common"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

// PrepareOSTask performs essential OS-level configurations on target nodes.
type PrepareOSTask struct {
	task.BaseTask
}

// NewPrepareOSTask creates a new PrepareOSTask.
func NewPrepareOSTask() task.Task {
	return &PrepareOSTask{
		BaseTask: task.NewBaseTask(
			"PrepareNodeOS",
			"Performs essential OS configurations for Kubernetes.",
			[]string{common.RoleMaster, common.RoleWorker},
			nil,
			false,
		),
	}
}

func (t *PrepareOSTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	allHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}
	if len(allHosts) == 0 {
		return task.NewEmptyFragment(), nil
	}

	// --- Step 1: Gather Facts ---
	// This step should run first to get information about the hosts.
	gatherFactsStep := common.NewGatherFactsStep("GatherFacts")
	gatherFactsNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  gatherFactsStep.Meta().Name,
		Step:  gatherFactsStep,
		Hosts: allHosts,
	})

	// --- Step 2: Parallel OS Configuration ---
	clusterCfg := ctx.GetClusterConfig()
	var osConfigDependencies = []plan.NodeID{gatherFactsNodeID}
	var osConfigNodeIDs []plan.NodeID

	// Load Kernel Modules
	kernelModules := []string{"br_netfilter", "overlay", "ip_vs"}
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.Modules) > 0 {
		kernelModules = clusterCfg.Spec.System.Modules
	}
	loadModulesStep := osstep.NewLoadKernelModulesStep("LoadKernelModules", kernelModules, true, "")
	loadModulesNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         loadModulesStep.Meta().Name,
		Step:         loadModulesStep,
		Hosts:        allHosts,
		Dependencies: osConfigDependencies,
	})
	osConfigNodeIDs = append(osConfigNodeIDs, loadModulesNodeID)

	// Set Sysctl Parameters
	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
	}
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.SysctlParams) > 0 {
		sysctlParams = clusterCfg.Spec.System.SysctlParams
	}
	setSysctlStep := osstep.NewConfigureSysctlStep("SetSysctl", sysctlParams, true, "/etc/sysctl.d/90-kubexms-kernel.conf")
	setSysctlNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         setSysctlStep.Meta().Name,
		Step:         setSysctlStep,
		Hosts:        allHosts,
		Dependencies: osConfigDependencies,
	})
	osConfigNodeIDs = append(osConfigNodeIDs, setSysctlNodeID)

	// Disable Swap
	disableSwapStep := osstep.NewDisableSwapStep("DisableSwapOnNodes", true)
	disableSwapNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         disableSwapStep.Meta().Name,
		Step:         disableSwapStep,
		Hosts:        allHosts,
		Dependencies: osConfigDependencies,
	})
	osConfigNodeIDs = append(osConfigNodeIDs, disableSwapNodeID)

	// Configure SELinux
	selinuxMode := common.DefaultSELinuxMode
	if clusterCfg.Spec.System != nil && clusterCfg.Spec.System.SELinux != "" {
		selinuxMode = clusterCfg.Spec.System.SELinux
	}
	configureSELinuxStep := osstep.NewConfigureSELinuxStep("ConfigureSELinux", selinuxMode, true)
	configureSELinuxNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         configureSELinuxStep.Meta().Name,
		Step:         configureSELinuxStep,
		Hosts:        allHosts,
		Dependencies: osConfigDependencies,
	})
	osConfigNodeIDs = append(osConfigNodeIDs, configureSELinuxNodeID)

	// Disable Common Firewalls
	var targetFirewalls []string
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.TargetFirewalls) > 0 {
		targetFirewalls = clusterCfg.Spec.System.TargetFirewalls
	}
	disableFirewallStep := osstep.NewDisableFirewallStep("DisableFirewalls", true, targetFirewalls)
	disableFirewallNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         disableFirewallStep.Meta().Name,
		Step:         disableFirewallStep,
		Hosts:        allHosts,
		Dependencies: osConfigDependencies,
	})
	osConfigNodeIDs = append(osConfigNodeIDs, disableFirewallNodeID)

	// --- Step 3: Hostname and /etc/hosts ---
	// These should run after the main OS configuration is done.
	setHostnameStep := common.NewSetHostnameStep("SetHostname")
	setHostnameNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         setHostnameStep.Meta().Name,
		Step:         setHostnameStep,
		Hosts:        allHosts,
		Dependencies: osConfigNodeIDs,
	})

	addHostsStep := osstep.NewAddHostsStep("AddHosts")
	_, _ = fragment.AddNode(&plan.ExecutionNode{
		Name:         addHostsStep.Meta().Name,
		Step:         addHostsStep,
		Hosts:        allHosts,
		Dependencies: []plan.NodeID{setHostnameNodeID}, // Depends on hostname being set
	})

	fragment.CalculateEntryAndExitNodes()
	logger.Info("OS preparation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*PrepareOSTask)(nil)
