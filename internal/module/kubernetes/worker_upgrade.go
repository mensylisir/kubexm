package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	taskKubeadm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

type WorkerUpgradeModule struct {
	module.BaseModule
	targetVersion string
}

func NewWorkerUpgradeModule(targetVersion string) module.Module {
	return &WorkerUpgradeModule{
		BaseModule:    module.NewBaseModule("WorkerUpgrade", nil),
		targetVersion: targetVersion,
	}
}

func (m *WorkerUpgradeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
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

	if kubeType == string(common.KubernetesDeploymentTypeKubeadm) {
		upgradeTask := taskKubeadm.NewUpgradeWorkersTask(m.targetVersion)
		moduleFragment, err := m.PlanSingleTask(taskCtx, upgradeTask)
		if err != nil {
			return nil, fmt.Errorf("failed to plan worker upgrade: %w", err)
		}
		logger.Info("WorkerUpgrade module planning complete", "total_nodes", len(moduleFragment.Nodes))
		return moduleFragment, nil
	} else {
		// kubexm type worker upgrade is not fully implemented
		logger.Warn("Kubexm worker upgrade logic is not fully implemented.")
		return plan.NewEmptyFragment(m.Name()), nil
	}
}

var _ module.Module = (*WorkerUpgradeModule)(nil)
