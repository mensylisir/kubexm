package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeadm2 "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type RolloutEtcdCertsCATask struct {
	task.Base
}

func NewRolloutEtcdCertsCATask() task.Task {
	return &RolloutEtcdCertsCATask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RolloutEtcdCaCerts",
				Description: "Distributes the Etcd CA bundle to all nodes and performs a rolling restart",
			},
		},
	}
}

func (t *RolloutEtcdCertsCATask) Name() string {
	return t.Meta.Name
}

func (t *RolloutEtcdCertsCATask) Description() string {
	return t.Meta.Description
}

func (t *RolloutEtcdCertsCATask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if val, ok := ctx.GetPipelineCache().Get(common.CacheKubeadmEtcdCACertRenew); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	return false, nil
}

func (t *RolloutEtcdCertsCATask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("no etcd hosts found, cannot plan rollout task")
	}
	opHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range etcdHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		backupStep := kubeadm.NewKubeadmBackupRemotePKIStepBuilder(*runtimeCtx, fmt.Sprintf("BackupFor%s", hostName)).Build()
		distributeStep := kubeadm.NewKubeadmDistributeStackedEtcdPKIStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeCAFor%s", hostName)).Build()
		restartStep := kubeadm.NewKubeadmRestartEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("RestartEtcdFor%s", hostName)).Build()
		verifyPodStep := kubeadm2.NewKubeadmVerifyEtcdPodHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyEtcdPodFor%s", hostName)).Build()

		backupNode := &plan.ExecutionNode{Name: backupStep.Meta().Name, Step: backupStep, Hosts: hostList}
		distributeNode := &plan.ExecutionNode{Name: distributeStep.Meta().Name, Step: distributeStep, Hosts: hostList}
		restartNode := &plan.ExecutionNode{Name: restartStep.Meta().Name, Step: restartStep, Hosts: hostList}
		verifyPodNode := &plan.ExecutionNode{Name: verifyPodStep.Meta().Name, Step: verifyPodStep, Hosts: hostList}

		backupID, _ := fragment.AddNode(backupNode)
		distributeID, _ := fragment.AddNode(distributeNode)
		restartID, _ := fragment.AddNode(restartNode)
		verifyPodID, _ := fragment.AddNode(verifyPodNode)

		fragment.AddDependency(backupID, distributeID)
		fragment.AddDependency(distributeID, restartID)
		fragment.AddDependency(restartID, verifyPodID)

		if lastNodeExitPoint != "" {
			_ = fragment.AddDependency(lastNodeExitPoint, backupID)
		}

		lastNodeExitPoint = verifyPodID
	}

	verifyClusterStep := kubeadm2.NewKubeadmVerifyEtcdClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealth").Build()
	verifyClusterNode := &plan.ExecutionNode{Name: verifyClusterStep.Meta().Name, Step: verifyClusterStep, Hosts: []connector.Host{opHost}}
	verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

	if lastNodeExitPoint != "" {
		_ = fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
