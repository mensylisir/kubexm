package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/health"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type RolloutLeafCertsToMastersTask struct {
	task.Base
}

func NewRolloutLeafCertsToMastersTask() task.Task {
	return &RolloutLeafCertsToMastersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RolloutLeafCertsToMasters",
				Description: "Distributes new leaf certificates to all masters and performs a rolling restart of the control plane",
			},
		},
	}
}

func (t *RolloutLeafCertsToMastersTask) Name() string {
	return t.Meta.Name
}

func (t *RolloutLeafCertsToMastersTask) Description() string {
	return t.Meta.Description
}

func (t *RolloutLeafCertsToMastersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	var renewalTriggered bool
	if val, ok := ctx.GetModuleCache().Get(kubexmstep.CacheKeyK8sCARequiresRenewal); ok {
		if renew, isBool := val.(bool); isBool && renew {
			renewalTriggered = true
		}
	}
	if !renewalTriggered {
		if val, ok := ctx.GetModuleCache().Get(kubexmstep.CacheKeyK8sLeafCertsRequireRenewal); ok {
			if renew, isBool := val.(bool); isBool && renew {
				renewalTriggered = true
			}
		}
	}
	return renewalTriggered, nil
}

func (t *RolloutLeafCertsToMastersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found for task %s", t.Name())
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range masterHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		backupStep := kubexmstep.NewKubexmBackupRemotePKIStepBuilder(*runtimeCtx, fmt.Sprintf("BackupLeafsPKIFor%s", hostName)).Build()
		distributeStep := kubexmstep.NewDistributeLeafCertsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeLeafsFor%s", hostName)).Build()
		restartApiServerStep := kube_apiserver.NewRestartKubeApiServerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartApiServerForLeafs%s", hostName)).Build()
		verifyApiServerStep := health.NewVerifyAPIServerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyApiServerForLeafs%s", hostName)).Build()
		restartControllerManagerStep := kube_controller_manager.NewRestartKubeControllerManagerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartControllerManagerForLeafs%s", hostName)).Build()
		verifyControllerManagerStep := health.NewVerifyControllerManagerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyControllerManagerForLeafs%s", hostName)).Build()
		restartSchedulerStep := kube_scheduler.NewRestartKubeSchedulerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartSchedulerForLeafs%s", hostName)).Build()
		verifySchedulerStep := health.NewVerifySchedulerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifySchedulerForLeafs%s", hostName)).Build()

		backupNode := &plan.ExecutionNode{Name: fmt.Sprintf("BackupLeafsPKI_%s", hostName), Step: backupStep, Hosts: hostList}
		distributeNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeLeafCerts_%s", hostName), Step: distributeStep, Hosts: hostList}
		restartApiServerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartApiServer_ForLeafs_%s", hostName), Step: restartApiServerStep, Hosts: hostList}
		verifyApiServerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyApiServer_ForLeafs_%s", hostName), Step: verifyApiServerStep, Hosts: hostList}
		restartControllerManagerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartControllerManager_ForLeafs_%s", hostName), Step: restartControllerManagerStep, Hosts: hostList}
		verifyControllerManagerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyControllerManager_ForLeafs_%s", hostName), Step: verifyControllerManagerStep, Hosts: hostList}
		restartSchedulerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartScheduler_ForLeafs_%s", hostName), Step: restartSchedulerStep, Hosts: hostList}
		verifySchedulerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyScheduler_ForLeafs_%s", hostName), Step: verifySchedulerStep, Hosts: hostList}

		backupID, _ := fragment.AddNode(backupNode)
		distributeID, _ := fragment.AddNode(distributeNode)
		restartApiServerID, _ := fragment.AddNode(restartApiServerNode)
		verifyApiServerID, _ := fragment.AddNode(verifyApiServerNode)
		restartControllerManagerID, _ := fragment.AddNode(restartControllerManagerNode)
		verifyControllerManagerID, _ := fragment.AddNode(verifyControllerManagerNode)
		restartSchedulerID, _ := fragment.AddNode(restartSchedulerNode)
		verifySchedulerID, _ := fragment.AddNode(verifySchedulerNode)

		fragment.AddDependency(backupID, distributeID)
		fragment.AddDependency(distributeID, restartApiServerID)
		fragment.AddDependency(restartApiServerID, verifyApiServerID)
		fragment.AddDependency(verifyApiServerID, restartControllerManagerID)
		fragment.AddDependency(restartControllerManagerID, verifyControllerManagerID)
		fragment.AddDependency(verifyControllerManagerID, restartSchedulerID)
		fragment.AddDependency(restartSchedulerID, verifySchedulerID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, backupID)
		}

		lastNodeExitPoint = verifySchedulerID
	}

	verifyClusterStep := health.NewCheckClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealthAfterLeafsRollout").Build()
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

var _ task.Task = (*RolloutLeafCertsToMastersTask)(nil)
