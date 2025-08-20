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

type RolloutEtcdLeafCertsTask struct {
	task.Base
}

func NewRolloutEtcdLeafCertsTask() task.Task {
	return &RolloutEtcdLeafCertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RolloutEtcdLeafCerts",
				Description: "Distributes the new Etcd leaf certificates to all nodes and performs a rolling restart",
			},
		},
	}
}

func (t *RolloutEtcdLeafCertsTask) Name() string {
	return t.Meta.Name
}

func (t *RolloutEtcdLeafCertsTask) Description() string {
	return t.Meta.Description
}

func (t *RolloutEtcdLeafCertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {

	if val, ok := ctx.GetPipelineCache().Get(kubeadm.CacheKeyK8sCARequiresRenewal); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	if val, ok := ctx.GetPipelineCache().Get(kubeadm.CacheKeyStackedEtcdLeafCertsRequireRenewal); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}

	return false, nil
}

func (t *RolloutEtcdLeafCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("no etcd hosts found, cannot plan rollout task")
	}
	opHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	var caRequiresRenewal bool
	if val, ok := ctx.GetPipelineCache().Get(kubeadm.CacheKeyK8sCARequiresRenewal); ok {
		if renew, isBool := val.(bool); isBool {
			caRequiresRenewal = renew
		}
	}

	var lastNodeExitPoint plan.NodeID = ""

	if caRequiresRenewal {
		moveStep := kubeadm.NewKubeadmMoveNewAssetsStepBuilder(*runtimeCtx, "MoveNewEtcdAssets").Build()
		moveNode := &plan.ExecutionNode{Name: "MoveNewEtcdAssets", Step: moveStep, Hosts: []connector.Host{opHost}}
		moveID, err := fragment.AddNode(moveNode)
		if err != nil {
			return nil, err
		}
		lastNodeExitPoint = moveID
	}

	for _, host := range etcdHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		backupStep := kubeadm.NewKubeadmBackupRemotePKIStepBuilder(*runtimeCtx, fmt.Sprintf("BackupFor%s", hostName)).Build()
		distributeStep := kubeadm.NewKubeadmDistributeStackedEtcdLeafCertsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeLeafsFor%s", hostName)).Build()
		restartStep := kubeadm.NewKubeadmRestartEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("RestartEtcdFor%s", hostName)).Build()
		verifyPodStep := kubeadm2.NewKubeadmVerifyEtcdPodHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyPodFor%s", hostName)).Build()

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
