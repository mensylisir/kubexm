package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	stepcommon "github.com/mensylisir/kubexm/internal/step/common"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/health"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kube-apiserver"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kube-controller-manager"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	kubexmstep "github.com/mensylisir/kubexm/internal/step/pki/kubexm"
	"github.com/mensylisir/kubexm/internal/task"
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
	runtimeCtx := ctx.(*runtime.Context)
	caCacheKey := fmt.Sprintf(common.CacheKubexmK8sCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	if val, ok := ctx.GetModuleCache().Get(caCacheKey); ok {
		if renew, isBool := val.(bool); isBool && renew {
			renewalTriggered = true
		}
	}
	if !renewalTriggered {
		leafCacheKey := fmt.Sprintf(common.CacheKubexmK8sLeafCertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
		if val, ok := ctx.GetModuleCache().Get(leafCacheKey); ok {
			if renew, isBool := val.(bool); isBool && renew {
				renewalTriggered = true
			}
		}
	}
	return renewalTriggered, nil
}

func (t *FinalizeMastersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
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

		backupStep, err := kubexmstep.NewBackupKubeconfigsStepBuilder(runtimeCtx, fmt.Sprintf("BackupMasterConfigsFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		distributeCAStep, err := kubexmstep.NewKubexmDistributeK8sPKIStepBuilder(runtimeCtx, fmt.Sprintf("DistributeFinalCAFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		distributeMasterKubeconfigsStep, err := kubexmstep.NewDistributeMasterKubeconfigsStepBuilder(runtimeCtx, fmt.Sprintf("DistributeMasterKubeconfigsFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		restartKubeletStep, err := kubelet.NewRestartKubeletStepBuilder(runtimeCtx, fmt.Sprintf("RestartKubeletOnMasterFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifyKubeletStep, err := health.NewVerifyKubeletHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifyKubeletOnMasterFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		restartKubeProxyStep, err := kube_proxy.NewRestartKubeProxyStepBuilder(runtimeCtx, fmt.Sprintf("RestartKubeProxyOnMasterFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifyKubeProxyStep, err := health.NewVerifyKubeProxyHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifyKubeProxyOnMasterFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		restartApiServerStep, err := kube_apiserver.NewRestartKubeApiServerStepBuilder(runtimeCtx, fmt.Sprintf("RestartApiServerFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifyApiServerStep, err := health.NewVerifyAPIServerHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifyApiServerFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		restartCMStep, err := kube_controller_manager.NewRestartKubeControllerManagerStepBuilder(runtimeCtx, fmt.Sprintf("RestartControllerManagerFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifyCMStep, err := health.NewVerifyControllerManagerHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifyControllerManagerFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		restartSchedulerStep, err := kube_scheduler.NewRestartKubeSchedulerStepBuilder(runtimeCtx, fmt.Sprintf("RestartSchedulerFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifySchedulerStep, err := health.NewVerifySchedulerHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifySchedulerFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		barrierStep, err := stepcommon.NewNoOpStepBuilder(fmt.Sprintf("BarrierFinalFor%s", hostName), "Wait for all components on master to be healthy").Build()
		if err != nil {
			return nil, err
		}

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
		verifyClusterStep, err := health.NewCheckClusterHealthStepBuilder(runtimeCtx, "UltimateClusterHealthVerification").Build()
		if err != nil {
			return nil, err
		}
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
