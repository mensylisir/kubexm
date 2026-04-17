package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeadmstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

// UpgradeWorkersTask handles upgrading Kubernetes worker nodes.
type UpgradeWorkersTask struct {
	task.Base
	targetVersion string
}

// NewUpgradeWorkersTask creates a new UpgradeWorkersTask.
func NewUpgradeWorkersTask(targetVersion string) task.Task {
	return &UpgradeWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "UpgradeWorkers",
				Description: fmt.Sprintf("Upgrades Kubernetes worker nodes to version %s", targetVersion),
			},
		},
		targetVersion: targetVersion,
	}
}

func (t *UpgradeWorkersTask) Name() string {
	return t.Meta.Name
}

func (t *UpgradeWorkersTask) Description() string {
	return t.Meta.Description
}

func (t *UpgradeWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Quick check: if there are no worker nodes, upgrade is not required
	workerNodes := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerNodes) == 0 {
		ctx.GetLogger().Info("No worker nodes found, skipping worker upgrade task")
		return false, nil
	}

	// Version check is handled per-host in the Step's Precheck
	// to avoid redundant SSH calls when only some nodes need upgrade
	return true, nil
}

func (t *UpgradeWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	workerNodes := ctx.GetHostsByRole(common.RoleWorker)

	if len(workerNodes) == 0 {
		ctx.GetLogger().Info("No worker nodes found, skipping upgrade")
		return fragment, nil
	}

	// Workers: run kubeadm upgrade node
	upgradeNodeStep, err := kubeadmstep.NewKubeadmUpgradeNodeStepBuilder(runtimeCtx, "UpgradeWorkerNode").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{
		Name:  "UpgradeWorkers",
		Step:  upgradeNodeStep,
		Hosts: workerNodes,
	})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*UpgradeWorkersTask)(nil)
