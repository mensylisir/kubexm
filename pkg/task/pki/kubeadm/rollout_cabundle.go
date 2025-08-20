package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeadmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type RolloutMasterCertsCATask struct {
	task.Base
}

func NewRolloutMasterCertsCATask() task.Task {
	return &RolloutMasterCertsCATask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RolloutUpdateMasterCA",
				Description: "Distributes the K8s CA to all masters and performs a rolling restart of the control plane",
			},
		},
	}
}

func (t *RolloutMasterCertsCATask) Name() string {
	return t.Meta.Name
}

func (t *RolloutMasterCertsCATask) Description() string {
	return t.Meta.Description
}

func (t *RolloutMasterCertsCATask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if val, ok := ctx.GetModuleCache().Get(kubeadmstep.CacheKeyK8sCARequiresRenewal); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	return false, nil
}

func (t *RolloutMasterCertsCATask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext's base context is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found, cannot plan bundle rollout task")
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range masterHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		backupPkiStep := kubeadmstep.NewKubeadmBackupRemotePKIStepBuilder(*runtimeCtx, fmt.Sprintf("BackupPKIFor%s", hostName)).Build()
		distributeBundleStep := kubeadmstep.NewKubeadmDistributeK8sPKIStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeBundleFor%s", hostName)).Build()
		restartControlPlaneStep := kubeadmstep.NewKubeadmRestartControlPlaneStepBuilder(*runtimeCtx, fmt.Sprintf("RestartControlPlaneFor%s", hostName)).Build()
		verifyControlPlaneStep := kubeadm.NewKubeadmVerifyControlPlaneHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyControlPlaneFor%s", hostName)).Build()

		backupPkiNode := &plan.ExecutionNode{Name: backupPkiStep.Meta().Name, Step: backupPkiStep, Hosts: hostList}
		distributeBundleNode := &plan.ExecutionNode{Name: distributeBundleStep.Meta().Name, Step: distributeBundleStep, Hosts: hostList}
		restartControlPlaneNode := &plan.ExecutionNode{Name: restartControlPlaneStep.Meta().Name, Step: restartControlPlaneStep, Hosts: hostList}
		verifyControlPlaneNode := &plan.ExecutionNode{Name: verifyControlPlaneStep.Meta().Name, Step: verifyControlPlaneStep, Hosts: hostList}

		backupPkiID, _ := fragment.AddNode(backupPkiNode)
		distributeBundleID, _ := fragment.AddNode(distributeBundleNode)
		restartControlPlaneID, _ := fragment.AddNode(restartControlPlaneNode)
		verifyControlPlaneID, _ := fragment.AddNode(verifyControlPlaneNode)

		fragment.AddDependency(backupPkiID, distributeBundleID)
		fragment.AddDependency(distributeBundleID, restartControlPlaneID)
		fragment.AddDependency(restartControlPlaneID, verifyControlPlaneID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, backupPkiID)
		}

		lastNodeExitPoint = verifyControlPlaneID
	}

	verifyClusterStep := kubeadm.NewKubeadmVerifyClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealthAfterBundleRollout").Build()
	verifyClusterNode := &plan.ExecutionNode{
		Name:  verifyClusterStep.Meta().Name,
		Step:  verifyClusterStep,
		Hosts: []connector.Host{masterHosts[0]},
	}
	verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

	if lastNodeExitPoint != "" {
		fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*RolloutMasterCertsCATask)(nil)
