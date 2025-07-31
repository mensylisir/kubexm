package preflight

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
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

	clusterCfg := ctx.GetClusterConfig()
	var lastStepID plan.NodeID

	// --- Step 1: Disable Swap ---
	disableSwapStep := osstep.NewDisableSwapStep("DisableSwapOnNodes", true)
	lastStepID, _ = fragment.AddNode(&plan.ExecutionNode{
		Name:  disableSwapStep.Meta().Name,
		Step:  disableSwapStep,
		Hosts: allHosts,
	})

	// --- Step 2: Configure SELinux ---
	selinuxMode := common.DefaultSELinuxMode
	if clusterCfg.Spec.System != nil && clusterCfg.Spec.System.SELinux != "" {
		selinuxMode = clusterCfg.Spec.System.SELinux
	}
	configureSELinuxStep := osstep.NewConfigureSELinuxStep("ConfigureSELinux", selinuxMode, true)
	lastStepID, _ = fragment.AddNode(&plan.ExecutionNode{
		Name:         configureSELinuxStep.Meta().Name,
		Step:         configureSELinuxStep,
		Hosts:        allHosts,
		Dependencies: []plan.NodeID{lastStepID},
	})

	// --- Step 3: Disable Common Firewalls ---
	var targetFirewalls []string
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.TargetFirewalls) > 0 {
		targetFirewalls = clusterCfg.Spec.System.TargetFirewalls
	}
	disableFirewallStep := osstep.NewDisableFirewallStep("DisableFirewalls", true, targetFirewalls)
	lastStepID, _ = fragment.AddNode(&plan.ExecutionNode{
		Name:         disableFirewallStep.Meta().Name,
		Step:         disableFirewallStep,
		Hosts:        allHosts,
		Dependencies: []plan.NodeID{lastStepID},
	})

	// --- Step 4: Set IPTables Alternatives ---
	iptablesMode := common.DefaultIPTablesMode
	if clusterCfg.Spec.System != nil && clusterCfg.Spec.System.IPTablesMode != "" {
		iptablesMode = clusterCfg.Spec.System.IPTablesMode
	}
	setIPTablesAltStep := osstep.NewSetIPTablesAlternativesStep("SetIPTablesAlternatives", iptablesMode, true)
	lastStepID, _ = fragment.AddNode(&plan.ExecutionNode{
		Name:         setIPTablesAltStep.Meta().Name,
		Step:         setIPTablesAltStep,
		Hosts:        allHosts,
		Dependencies: []plan.NodeID{lastStepID},
	})

	// --- Step 5: Update /etc/hosts ---
	if clusterCfg.Spec.HostsFileContent != "" {
		etcHostsEntries := make(map[string][]string)
		lines := strings.Split(strings.TrimSpace(clusterCfg.Spec.HostsFileContent), "\n")
		for _, line := range lines {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) >= 2 {
				ip := fields[0]
				hostnames := fields[1:]
				etcHostsEntries[ip] = append(etcHostsEntries[ip], hostnames...)
			}
		}

		if len(etcHostsEntries) > 0 {
			updateHostsStep := osstep.NewUpdateEtcHostsStep("UpdateEtcHostsFile", etcHostsEntries, true)
			lastStepID, _ = fragment.AddNode(&plan.ExecutionNode{
				Name:         updateHostsStep.Meta().Name,
				Step:         updateHostsStep,
				Hosts:        allHosts,
				Dependencies: []plan.NodeID{lastStepID},
			})
		}
	}

	// --- Step 6: Apply Security Limits ---
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.SecurityLimits) > 0 {
		applyLimitsStep := osstep.NewApplySecurityLimitsStep("ApplySecurityLimits", clusterCfg.Spec.System.SecurityLimits, "", true)
		lastStepID, _ = fragment.AddNode(&plan.ExecutionNode{
			Name:         applyLimitsStep.Meta().Name,
			Step:         applyLimitsStep,
			Hosts:        allHosts,
			Dependencies: []plan.NodeID{lastStepID},
		})
	}

	fragment.CalculateEntryAndExitNodes()
	logger.Info("OS preparation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*PrepareOSTask)(nil)
