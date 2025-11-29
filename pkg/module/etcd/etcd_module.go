package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskEtcd "github.com/mensylisir/kubexm/pkg/task/etcd" // Renamed to avoid conflict
)

// EtcdModule manages the ETCD cluster setup.
type EtcdModule struct {
	module.BaseModule
}

// NewEtcdModule creates a new module for ETCD cluster management.
func NewEtcdModule() module.Module {
	// Instantiate tasks that this module will manage.
	// The InstallETCDTask is the primary one for setting up ETCD.
	// Other tasks like backup, restore, member management could be added here later.
	installEtcdTask := taskEtcd.NewInstallETCDTask()

	// PKI related tasks might also be part of this module or a separate PKI module.
	// For now, let's assume InstallETCDTask handles what's needed or depends on a PKI task.
	// If PKI generation is complex and separate:
	//   pkiSetupTask := taskPki.NewSetupEtcdPkiTask() // Example name
	//   allModuleTasks := []task.Task{pkiSetupTask, installEtcdTask}

	allModuleTasks := []task.Task{installEtcdTask}

	base := module.NewBaseModule("ETCDClusterManagement", allModuleTasks)
	return &EtcdModule{BaseModule: base}
}

// Plan generates the execution fragment for the ETCD module.
// It calls the Plan method of its constituent tasks and links them if necessary.
func (m *EtcdModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Module level enablement check: only proceed if ETCD is configured at all.
	// The InstallETCDTask itself will further check if it's an internal type.
	if clusterConfig.Spec.Etcd == nil {
		logger.Info("ETCD module is not required (ETCD spec is nil). Skipping.")
		return plan.NewEmptyFragment(m.Name()), nil
	}
	// If type is external, InstallETCDTask.IsRequired will be false, and this module will produce an empty fragment.
	// A more advanced EtcdModule could include tasks for setting up etcd client certs on masters even for external etcd.

	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment") // Named fragment
	var previousTaskExitNodes []plan.NodeID

	// The EtcdModule should also include the GenerateEtcdPkiTask if it's responsible for its own PKI.
	// This task runs on the control node.
	// TODO: Get AltNameHosts, CPEndpoint, DefaultLBDomain from clusterConfig or runtime.Context
	// For now, assuming these are empty or handled by GenerateEtcdPkiTask defaults.
	// This implies GenerateEtcdPkiTask needs access to clusterConfig.
	var etcdHostsForAltNames []taskEtcd.HostSpecForPki // Corrected type
	// Populate etcdHostsForAltNames from ctx.GetHostsByRole(v1alpha1.ETCDRole)

	generatePkiTask := taskEtcd.NewGenerateEtcdPkiTask(etcdHostsForAltNames, clusterConfig.Spec.ControlPlaneEndpoint.Domain, "lb.kubexm.internal") // Example values

	// Assert ModuleContext to TaskContext for calling Task methods
	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for EtcdModule")
	}

	pkiTaskRequired, err := generatePkiTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if GenerateEtcdPkiTask is required: %w", err)
	}

	if pkiTaskRequired {
		logger.Info("Planning GenerateEtcdPkiTask")
		pkiFragment, err := generatePkiTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan GenerateEtcdPkiTask: %w", err)
		}
		if err := moduleFragment.MergeFragment(pkiFragment); err != nil {
			return nil, fmt.Errorf("failed to merge PKI fragment into EtcdModule: %w", err)
		}
		// All nodes in pkiFragment are entry nodes for the module initially
		moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, pkiFragment.EntryNodes...)
		previousTaskExitNodes = pkiFragment.ExitNodes
	}


	// Now plan the InstallETCDTask, which depends on the PKI task.
	installEtcdTask := taskEtcd.NewInstallETCDTask() // This is one of the tasks from m.Tasks()

	installTaskRequired, err := installEtcdTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if InstallETCDTask is required: %w", err)
	}

	if installTaskRequired {
		logger.Info("Planning InstallETCDTask")
		installFragment, err := installEtcdTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan InstallETCDTask: %w", err)
		}
		if err := moduleFragment.MergeFragment(installFragment); err != nil {
			return nil, fmt.Errorf("failed to merge InstallETCDTask fragment into EtcdModule: %w", err)
		}

		// Link InstallETCDTask's entry nodes to GenerateEtcdPkiTask's exit nodes
		if len(previousTaskExitNodes) > 0 {
			for _, entryNodeID := range installFragment.EntryNodes {
				if entryNode, exists := moduleFragment.Nodes[entryNodeID]; exists {
					entryNode.Dependencies = append(entryNode.Dependencies, previousTaskExitNodes...)
					entryNode.Dependencies = plan.UniqueNodeIDs(entryNode.Dependencies)
				} else {
					return nil, fmt.Errorf("entry node ID '%s' from InstallETCDTask not found in module fragment", entryNodeID)
				}
			}
		} else { // PKI task was skipped, InstallETCDTask entries are module entries
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, installFragment.EntryNodes...)
		}
		previousTaskExitNodes = installFragment.ExitNodes // Update exit nodes for the module
	} else {
		// If install task is not required (e.g. external etcd), and PKI task was also skipped or had no nodes,
		// the module might be empty. If PKI ran, its exits are module exits.
	}

	moduleFragment.ExitNodes = previousTaskExitNodes // The exits of the last task become module exits.
	moduleFragment.EntryNodes = task.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = task.UniqueNodeIDs(moduleFragment.ExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("EtcdModule planned no executable nodes (e.g., external etcd and no local PKI actions).")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("ETCD module planning complete.", "total_nodes", len(moduleFragment.Nodes), "entry_nodes", len(moduleFragment.EntryNodes), "exit_nodes", len(moduleFragment.ExitNodes))
	return moduleFragment, nil
}

// Ensure EtcdModule implements the module.Module interface.
var _ module.Module = (*EtcdModule)(nil)
