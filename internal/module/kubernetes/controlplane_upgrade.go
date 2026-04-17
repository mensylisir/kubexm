package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	taskKubeadm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

type ControlPlaneUpgradeModule struct {
	module.BaseModule
	targetVersion string
}

func NewControlPlaneUpgradeModule(targetVersion string) module.Module {
	return &ControlPlaneUpgradeModule{
		BaseModule:    module.NewBaseModule("ControlPlaneUpgrade", nil),
		targetVersion: targetVersion,
	}
}

func (m *ControlPlaneUpgradeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "target_version", m.targetVersion)

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	clusterCfg := taskCtx.GetClusterConfig()
	kubeType := clusterCfg.Spec.Kubernetes.Type
	if kubeType == "" {
		kubeType = string(common.KubernetesDeploymentTypeKubeadm)
	}

	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")
	var previousTaskExitNodes []plan.NodeID

	if kubeType == string(common.KubernetesDeploymentTypeKubeadm) {
		// For kubeadm, we need to ensure binaries are distributed BEFORE upgrading.
		// Ideally, we would distribute the kubeadm binary here.
		// For now, we rely on the UpgradeControlPlaneTask which assumes binaries are present.
		// TODO: Add binary distribution steps for targetVersion.
		
		upgradeTask := taskKubeadm.NewUpgradeControlPlaneTask(m.targetVersion)
		frag, err := upgradeTask.Plan(taskCtx)
		if err != nil {
			return nil, err
		}
		if err := moduleFragment.MergeFragment(frag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, frag.EntryNodes)
		}
		previousTaskExitNodes = frag.ExitNodes
	} else {
		// kubexm type upgrade is not fully implemented
		logger.Warn("Kubexm control plane upgrade logic is not fully implemented.")
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("ControlPlaneUpgrade module planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("ControlPlaneUpgrade module planning complete", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*ControlPlaneUpgradeModule)(nil)
