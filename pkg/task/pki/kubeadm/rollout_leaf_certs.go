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

type RolloutMasterLeafCertsTask struct {
	task.Base
}

func NewRolloutMasterLeafCertsTask() task.Task {
	return &RolloutMasterLeafCertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DistributeLeafCertsToMasters",
				Description: "Distributes new leaf certificates to all masters and performs a rolling restart of the control plane",
			},
		},
	}
}

func (t *RolloutMasterLeafCertsTask) Name() string {
	return t.Meta.Name
}

func (t *RolloutMasterLeafCertsTask) Description() string {
	return t.Meta.Description
}

func (t *RolloutMasterLeafCertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	var renewalTriggered bool
	if val, ok := ctx.GetModuleCache().Get(common.CacheKubeadmK8sCACertRenew); ok {
		if renew, isBool := val.(bool); isBool && renew {
			renewalTriggered = true
		}
	}
	if !renewalTriggered {
		if val, ok := ctx.GetModuleCache().Get(common.CacheKubeadmK8sLeafCertRenew); ok {
			if renew, isBool := val.(bool); isBool && renew {
				renewalTriggered = true
			}
		}
	}
	return renewalTriggered, nil
}

func (t *RolloutMasterLeafCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found for task %s", t.Name())
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range masterHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		distributeStep := kubeadmstep.NewKubeadmDistributeLeafCertsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeLeafs-For-%s", hostName)).Build()
		restartStep := kubeadmstep.NewKubeadmRestartControlPlaneStepBuilder(*runtimeCtx, fmt.Sprintf("RestartCP-ForLeafs-%s", hostName)).Build()
		verifyStep := kubeadm.NewKubeadmVerifyControlPlaneHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyCP-ForLeafs-%s", hostName)).Build()

		distributeNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeLeafCerts_%s", hostName), Step: distributeStep, Hosts: hostList}
		restartNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartCP_ForLeafCerts_%s", hostName), Step: restartStep, Hosts: hostList}
		verifyNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyCP_ForLeafCerts_%s", hostName), Step: verifyStep, Hosts: hostList}

		distributeID, _ := fragment.AddNode(distributeNode)
		restartID, _ := fragment.AddNode(restartNode)
		verifyID, _ := fragment.AddNode(verifyNode)

		fragment.AddDependency(distributeID, restartID)
		fragment.AddDependency(restartID, verifyID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, distributeID)
		}

		lastNodeExitPoint = verifyID
	}

	verifyClusterStep := kubeadm.NewKubeadmVerifyClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealthAfterLeafsRollout").Build()
	verifyClusterNode := &plan.ExecutionNode{
		Name:  "VerifyClusterHealthAfterLeafsRollout",
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

var _ task.Task = (*RolloutMasterLeafCertsTask)(nil)
