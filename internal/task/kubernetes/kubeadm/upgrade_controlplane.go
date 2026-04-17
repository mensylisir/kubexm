package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeadmstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

// UpgradeControlPlaneTask handles upgrading the Kubernetes control plane.
type UpgradeControlPlaneTask struct {
	task.Base
	targetVersion string
}

// NewUpgradeControlPlaneTask creates a new UpgradeControlPlaneTask.
func NewUpgradeControlPlaneTask(targetVersion string) task.Task {
	return &UpgradeControlPlaneTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "UpgradeControlPlane",
				Description: fmt.Sprintf("Upgrades Kubernetes control plane to version %s", targetVersion),
			},
		},
		targetVersion: targetVersion,
	}
}

func (t *UpgradeControlPlaneTask) Name() string {
	return t.Meta.Name
}

func (t *UpgradeControlPlaneTask) Description() string {
	return t.Meta.Description
}

func (t *UpgradeControlPlaneTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Quick check: if there are no control plane nodes, upgrade is not required
	controlPlaneNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(controlPlaneNodes) == 0 {
		ctx.GetLogger().Info("No control plane nodes found, skipping control plane upgrade task")
		return false, nil
	}

	// Version check is handled per-host in the Step's Precheck
	// to avoid redundant SSH calls when only some nodes need upgrade
	return true, nil
}

func (t *UpgradeControlPlaneTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlPlaneNodes := ctx.GetHostsByRole(common.RoleMaster)

	if len(controlPlaneNodes) == 0 {
		ctx.GetLogger().Info("No control plane nodes found, skipping upgrade")
		return fragment, nil
	}

	firstMaster := controlPlaneNodes[0]
	otherMasters := controlPlaneNodes[1:]

	// First master: run kubeadm upgrade apply
	upgradeApplyStep, err := kubeadmstep.NewKubeadmUpgradeApplyStepBuilder(runtimeCtx, "UpgradeApply").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{
		Name:  "UpgradeFirstMaster",
		Step:  upgradeApplyStep,
		Hosts: []remotefw.Host{firstMaster},
	})

	// Other masters: run kubeadm upgrade node sequentially
	if len(otherMasters) > 0 {
		upgradeNodeStep, err := kubeadmstep.NewKubeadmUpgradeNodeStepBuilder(runtimeCtx, "UpgradeNode").Build()
		if err != nil {
			return nil, err
		}
		fragment.AddNode(&plan.ExecutionNode{
			Name:  "UpgradeOtherMasters",
			Step:  upgradeNodeStep,
			Hosts: otherMasters,
		})
		fragment.AddDependency("UpgradeFirstMaster", "UpgradeOtherMasters")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*UpgradeControlPlaneTask)(nil)
