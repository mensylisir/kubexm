package preflight

import (
	"fmt"
	"strings" // Added for HostsFileContent parsing

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common" // Still needed for RoleMaster etc.
	"github.com/mensylisir/kubexm/pkg/util"   // Added for NonEmptyNodeIDs
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	// preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight" // For any specific preflight steps
	"github.com/mensylisir/kubexm/pkg/task"
)

// SetupKubernetesPrerequisitesTask sets up common system prerequisites for Kubernetes nodes.
type SetupKubernetesPrerequisitesTask struct {
	task.BaseTask
	// Configuration can be accessed from ctx.GetClusterConfig() in Plan()
}

// NewSetupKubernetesPrerequisitesTask creates a new SetupKubernetesPrerequisitesTask.
// runOnRoles specifies which nodes this task should apply to.
func NewSetupKubernetesPrerequisitesTask(runOnRoles []string) task.Task {
	return &SetupKubernetesPrerequisitesTask{
		BaseTask: task.NewBaseTask(
			"SetupKubernetesPrerequisites",
			"Sets up common system prerequisites for Kubernetes nodes (e.g., disable swap, SELinux, firewall, iptables mode).",
			runOnRoles, // e.g., common.AllHostsRole
			nil,        // HostFilter
			false,      // IgnoreError (these are usually critical)
		),
	}
}

func (t *SetupKubernetesPrerequisitesTask) IsRequired(ctx task.TaskContext) (bool, error) {
	// This task is generally always required for new node setups.
	// Specific steps within might have their own prechecks.
	if len(t.BaseTask.RunOnRoles) == 0 {
		ctx.GetLogger().Info("No target roles specified for SetupKubernetesPrerequisites task, skipping.")
		return false, nil
	}
	return true, nil
}

func (t *SetupKubernetesPrerequisitesTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	taskFragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	targetHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...)
	if err != nil {
		return nil, fmt.Errorf("failed to get target hosts for task %s: %w", t.Name(), err)
	}
	if len(targetHosts) == 0 {
		logger.Info("No target hosts found for roles, skipping prerequisite setup.", "roles", t.BaseTask.RunOnRoles)
		return task.NewEmptyFragment(), nil
	}

	var lastStepPerHost = make(map[string]plan.NodeID) // Tracks the last step for each host to chain dependencies

	// Initialize lastStepPerHost for all target hosts with a conceptual entry point if needed,
	// or leave empty if first steps for each host can be parallel module entries.
	// For this task, steps on each host are largely independent of other hosts, but sequential on a given host.

	var lastGlobalStepID plan.NodeID // Tracks the last globally applied step for sequencing

	// --- Step 1: Disable Swap ---
	if clusterCfg.Spec.Preflight == nil || clusterCfg.Spec.Preflight.DisableSwap == nil || *clusterCfg.Spec.Preflight.DisableSwap {
		disableSwapStep := osstep.NewDisableSwapStep("DisableSwapOnNodes", true)
		disableSwapNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
			Name:  disableSwapStep.Meta().Name, Step: disableSwapStep, Hosts: targetHosts,
		})
		lastGlobalStepID = disableSwapNodeID
	} else {
		logger.Info("Skipping swap disable as per PreflightConfig.")
	}

	// --- Step 2: Configure SELinux ---
	selinuxMode := common.DefaultSELinuxMode // Default to "permissive" or "disabled"
	if clusterCfg.Spec.System != nil && clusterCfg.Spec.System.SELinux != "" {
		selinuxMode = clusterCfg.Spec.System.SELinux
	}
	configureSELinuxStep := osstep.NewConfigureSELinuxStep("ConfigureSELinux", selinuxMode, true)
	configureSELinuxNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  configureSELinuxStep.Meta().Name, Step: configureSELinuxStep, Hosts: targetHosts,
		Dependencies: util.NonEmptyNodeIDs(lastGlobalStepID),
	})
	lastGlobalStepID = configureSELinuxNodeID

	// --- Step 3: Disable Common Firewalls ---
	// TargetFirewalls can be configured in SystemSpec if needed, otherwise uses defaults in step
	var targetFirewalls []string
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.TargetFirewalls) > 0 { // Assuming SystemSpec has TargetFirewalls
		targetFirewalls = clusterCfg.Spec.System.TargetFirewalls
	}
	disableFirewallStep := osstep.NewDisableFirewallStep("DisableFirewalls", true, targetFirewalls)
	disableFirewallNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  disableFirewallStep.Meta().Name, Step: disableFirewallStep, Hosts: targetHosts,
		Dependencies: util.NonEmptyNodeIDs(lastGlobalStepID),
	})
	lastGlobalStepID = disableFirewallNodeID

	// --- Step 4: Set IPTables Alternatives ---
	iptablesMode := common.DefaultIPTablesMode // Default to "legacy"
	if clusterCfg.Spec.System != nil && clusterCfg.Spec.System.IPTablesMode != "" {
		iptablesMode = clusterCfg.Spec.System.IPTablesMode
	}
	setIPTablesAltStep := osstep.NewSetIPTablesAlternativesStep("SetIPTablesAlternatives", iptablesMode, true)
	setIPTablesAltNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  setIPTablesAltStep.Meta().Name, Step: setIPTablesAltStep, Hosts: targetHosts,
		Dependencies: util.NonEmptyNodeIDs(lastGlobalStepID),
	})
	lastGlobalStepID = setIPTablesAltNodeID

	// --- Step 5: Update /etc/hosts (optional, based on config) ---
	if clusterCfg.Spec.HostsFileContent != "" {
		// Convert string to map[string][]string; assumes specific format or needs parsing logic.
		// For simplicity, assume HostsFileContent is a map or this task/step handles parsing.
		// This example assumes a simple case or direct map. A real parser for multiline string would be needed.
		// For now, let's make a dummy entry.
		// TODO: Implement proper parsing for HostsFileContent string from v1alpha1.ClusterSpec.
		etcHostsEntries := make(map[string][]string)
		// Example: etcHostsEntries["10.0.0.1"] = []string{"my-service.local"}
		if len(clusterCfg.Spec.HostsFileContentEntries) > 0 { // Assuming a new structured field
			etcHostsEntries = clusterCfg.Spec.HostsFileContentEntries
		} else if clusterCfg.Spec.HostsFileContent != "" {
		    // Basic parsing for multiline string if HostsFileContentEntries is not used
		    lines := strings.Split(strings.TrimSpace(clusterCfg.Spec.HostsFileContent), "\n")
		    for _, line := range lines {
			    fields := strings.Fields(strings.TrimSpace(line))
			    if len(fields) >= 2 {
				    ip := fields[0]
				    hostnames := fields[1:]
				    etcHostsEntries[ip] = append(etcHostsEntries[ip], hostnames...)
			    }
		    }
		}


		if len(etcHostsEntries) > 0 {
			updateHostsStep := osstep.NewUpdateEtcHostsStep("UpdateEtcHostsFile", etcHostsEntries, true)
			updateHostsNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
				Name:  updateHostsStep.Meta().Name, Step: updateHostsStep, Hosts: targetHosts,
				Dependencies: util.NonEmptyNodeIDs(lastGlobalStepID),
			})
			lastGlobalStepID = updateHostsNodeID
		}
	}

	// --- Step 6: Apply Security Limits (optional, based on config) ---
	if clusterCfg.Spec.System != nil && len(clusterCfg.Spec.System.SecurityLimits) > 0 { // Assuming SystemSpec has SecurityLimits
		applyLimitsStep := osstep.NewApplySecurityLimitsStep("ApplySecurityLimits", clusterCfg.Spec.System.SecurityLimits, "", true)
		applyLimitsNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
			Name:  applyLimitsStep.Meta().Name, Step: applyLimitsStep, Hosts: targetHosts,
			Dependencies: util.NonEmptyNodeIDs(lastGlobalStepID),
		})
		lastGlobalStepID = applyLimitsNodeID
	}

	taskFragment.CalculateEntryAndExitNodes()
	logger.Info("SetupKubernetesPrerequisitesTask planning complete.")
	return taskFragment, nil
}

var _ task.Task = (*SetupKubernetesPrerequisitesTask)(nil)
```
