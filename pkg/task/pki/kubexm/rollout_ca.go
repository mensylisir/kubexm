package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	stepcommon "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/health"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type FinalizeMastersTask struct {
	task.Base
}

func NewFinalizeMastersTask() task.Task {
	return &FinalizeMastersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "FinalizeMasters",
				Description: "Distributes all final certificates and kubeconfigs to masters and restarts all necessary components",
			},
		},
	}
}

func (t *FinalizeMastersTask) Name() string {
	return t.Meta.Name
}

func (t *FinalizeMastersTask) Description() string {
	return t.Meta.Description
}

func (t *FinalizeMastersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
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

func (t *FinalizeMastersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
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

		backupStep := kubexmstep.NewBackupKubeconfigsStepBuilder(*runtimeCtx, fmt.Sprintf("BackupMasterConfigsFor%s", hostName)).Build()
		distributeCAStep := kubexmstep.NewKubexmDistributeK8sPKIStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeFinalCAFor%s", hostName)).Build()
		distributeMasterKubeconfigsStep := kubexmstep.NewDistributeMasterKubeconfigsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeMasterKubeconfigsFor%s", hostName)).Build()

		restartKubeletStep := kubelet.NewRestartKubeletStepBuilder(*runtimeCtx, fmt.Sprintf("RestartKubeletOnMasterFor%s", hostName)).Build()
		verifyKubeletStep := health.NewVerifyKubeletHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyKubeletOnMasterFor%s", hostName)).Build()

		restartKubeProxyStep := kube_proxy.NewRestartKubeProxyStepBuilder(*runtimeCtx, fmt.Sprintf("RestartKubeProxyOnMasterFor%s", hostName)).Build()
		verifyKubeProxyStep := health.NewVerifyKubeProxyHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyKubeProxyOnMasterFor%s", hostName)).Build()

		restartApiServerStep := kube_apiserver.NewRestartKubeApiServerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartApiServerFinalFor%s", hostName)).Build()
		verifyApiServerStep := health.NewVerifyAPIServerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyApiServerFinalFor%s", hostName)).Build()
		restartCMStep := kube_controller_manager.NewRestartKubeControllerManagerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartControllerManagerFinalFor%s", hostName)).Build()
		verifyCMStep := health.NewVerifyControllerManagerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyControllerManagerFinalFor%s", hostName)).Build()
		restartSchedulerStep := kube_scheduler.NewRestartKubeSchedulerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartSchedulerFinalFor%s", hostName)).Build()
		verifySchedulerStep := health.NewVerifySchedulerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifySchedulerFinalFor%s", hostName)).Build()

		barrierStep := stepcommon.NewNoOpStepBuilder(fmt.Sprintf("BarrierFinalFor%s", hostName), "Wait for all components on master to be healthy").Build()

		backupNode := &plan.ExecutionNode{Name: fmt.Sprintf("BackupMasterConfigsFor%s", hostName), Step: backupStep, Hosts: hostList}
		distributeCANode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeFinalCAFor%s", hostName), Step: distributeCAStep, Hosts: hostList}
		distributeMasterKubeconfigsNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeMasterKubeconfigsFor%s", hostName), Step: distributeMasterKubeconfigsStep, Hosts: hostList}

		restartKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartKubeletOnMasterFor%s", hostName), Step: restartKubeletStep, Hosts: hostList}
		verifyKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyKubeletOnMasterFor%s", hostName), Step: verifyKubeletStep, Hosts: hostList}

		restartKubeProxyNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartKubeProxyOnMasterFor%s", hostName), Step: restartKubeProxyStep, Hosts: hostList}
		verifyKubeProxyNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyKubeProxyOnMasterFor%s", hostName), Step: verifyKubeProxyStep, Hosts: hostList}

		restartApiServerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartApiServerFinalFor%s", hostName), Step: restartApiServerStep, Hosts: hostList}
		verifyApiServerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyApiServerFinalFor%s", hostName), Step: verifyApiServerStep, Hosts: hostList}
		restartCMNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartControllerManagerFinalFor%s", hostName), Step: restartCMStep, Hosts: hostList}
		verifyCMNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyControllerManagerFinalFor%s", hostName), Step: verifyCMStep, Hosts: hostList}
		restartSchedulerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartSchedulerFinalFor%s", hostName), Step: restartSchedulerStep, Hosts: hostList}
		verifySchedulerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifySchedulerFinalFor%s", hostName), Step: verifySchedulerStep, Hosts: hostList}

		barrierNode := &plan.ExecutionNode{Name: fmt.Sprintf("MasterFinalBarrierFor%s", hostName), Step: barrierStep, Hosts: hostList}

		backupID, _ := fragment.AddNode(backupNode)
		distributeCAID, _ := fragment.AddNode(distributeCANode)
		distributeMasterKubeconfigsID, _ := fragment.AddNode(distributeMasterKubeconfigsNode)
		restartKubeletID, _ := fragment.AddNode(restartKubeletNode)
		verifyKubeletID, _ := fragment.AddNode(verifyKubeletNode)
		restartKubeProxyID, _ := fragment.AddNode(restartKubeProxyNode)
		verifyKubeProxyID, _ := fragment.AddNode(verifyKubeProxyNode)
		restartApiServerID, _ := fragment.AddNode(restartApiServerNode)
		verifyApiServerID, _ := fragment.AddNode(verifyApiServerNode)
		restartCMID, _ := fragment.AddNode(restartCMNode)
		verifyCMID, _ := fragment.AddNode(verifyCMNode)
		restartSchedulerID, _ := fragment.AddNode(restartSchedulerNode)
		verifySchedulerID, _ := fragment.AddNode(verifySchedulerNode)
		barrierID, _ := fragment.AddNode(barrierNode)

		fragment.AddDependency(backupID, distributeCAID)
		fragment.AddDependency(backupID, distributeMasterKubeconfigsID)

		fragment.AddDependency(distributeCAID, restartKubeletID)
		fragment.AddDependency(distributeMasterKubeconfigsID, restartKubeletID)
		fragment.AddDependency(distributeCAID, restartKubeProxyID)
		fragment.AddDependency(distributeMasterKubeconfigsID, restartKubeProxyID)

		fragment.AddDependency(restartKubeletID, verifyKubeletID)
		fragment.AddDependency(restartKubeProxyID, verifyKubeProxyID)

		fragment.AddDependency(distributeCAID, restartApiServerID)
		fragment.AddDependency(distributeMasterKubeconfigsID, restartApiServerID)
		fragment.AddDependency(verifyKubeletID, restartApiServerID)

		fragment.AddDependency(restartApiServerID, verifyApiServerID)
		fragment.AddDependency(verifyApiServerID, restartCMID)
		fragment.AddDependency(restartCMID, verifyCMID)
		fragment.AddDependency(verifyCMID, restartSchedulerID)
		fragment.AddDependency(restartSchedulerID, verifySchedulerID)

		fragment.AddDependency(verifyKubeletID, barrierID)
		fragment.AddDependency(verifyKubeProxyID, barrierID)
		fragment.AddDependency(verifySchedulerID, barrierID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, backupID)
		}

		lastNodeExitPoint = barrierID
	}

	if len(masterHosts) > 0 {
		verifyClusterStep := health.NewCheckClusterHealthStepBuilder(*runtimeCtx, "UltimateClusterHealthVerification").Build()
		verifyClusterNode := &plan.ExecutionNode{
			Name:  "UltimateClusterHealthVerification",
			Step:  verifyClusterStep,
			Hosts: []connector.Host{masterHosts[0]},
		}
		verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
		}
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*FinalizeMastersTask)(nil)
