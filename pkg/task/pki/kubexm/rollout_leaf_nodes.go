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
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type UpdateWorkerNodesTask struct {
	task.Base
}

func NewUpdateWorkerNodesTask() task.Task {
	return &UpdateWorkerNodesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "UpdateWorkerNodes",
				Description: "Distributes new CA and kubelet/kube-proxy configs to all worker nodes and restarts services",
			},
		},
	}
}

func (t *UpdateWorkerNodesTask) Name() string {
	return t.Meta.Name
}

func (t *UpdateWorkerNodesTask) Description() string {
	return t.Meta.Description
}

func (t *UpdateWorkerNodesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
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

func (t *UpdateWorkerNodesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)

	masterHostNames := make(map[string]bool)
	for _, master := range masterHosts {
		masterHostNames[master.GetName()] = true
	}

	pureWorkerHosts := make([]connector.Host, 0)
	for _, worker := range workerHosts {
		if !masterHostNames[worker.GetName()] {
			pureWorkerHosts = append(pureWorkerHosts, worker)
		}
	}

	if len(pureWorkerHosts) == 0 {
		ctx.GetLogger().Info("No pure worker nodes found to update. This task will do nothing.")
		return fragment, nil
	}

	ctx.GetLogger().Infof("Planning update for %d pure worker nodes.", len(pureWorkerHosts))

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range pureWorkerHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		backupKubeletStep := kubexmstep.NewKubexmBackupRemotePKIStepBuilder(*runtimeCtx, fmt.Sprintf("BackupKubeletConfigFor%s", hostName)).Build()
		distributeCertStep := kubexmstep.NewKubexmDistributeK8sPKIStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeWorkerCertsFor%s", hostName)).Build()
		distributeKubeconfigStep := kubexmstep.NewDistributeWorkerKubeconfigsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeWorkerKubeconfigsFor%s", hostName)).Build()
		restartKubeletStep := kubelet.NewRestartKubeletStepBuilder(*runtimeCtx, fmt.Sprintf("RestartKubeletFor%s", hostName)).Build()
		verifyKubeletStep := health.NewVerifyKubeletHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyWorkerFor%s", hostName)).Build()
		restartKubeProxyStep := kube_proxy.NewRestartKubeProxyStepBuilder(*runtimeCtx, fmt.Sprintf("RestartKubeProxyFor%s", hostName)).Build()
		verifyKubeProxyStep := health.NewVerifyKubeProxyHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyKubeProxyFor%s", hostName)).Build()

		backupKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("BackupKubeletConfigFor%s", hostName), Step: backupKubeletStep, Hosts: hostList}
		distributeCertsNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeWorkerCertsFor%s", hostName), Step: distributeCertStep, Hosts: hostList}
		distributeKubeconfigNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeWorkerKubeconfigsFor%s", hostName), Step: distributeKubeconfigStep, Hosts: hostList}
		restartKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartKubeletFor%s", hostName), Step: restartKubeletStep, Hosts: hostList}
		verifyKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyKubeletFor%s", hostName), Step: verifyKubeletStep, Hosts: hostList}
		restartKubeProxyNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartKubeProxyFor%s", hostName), Step: restartKubeProxyStep, Hosts: hostList}
		verifyKubeProxyNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyKubeletFor%s", hostName), Step: verifyKubeProxyStep, Hosts: hostList}

		backupKubeletID, _ := fragment.AddNode(backupKubeletNode)
		distributeCertID, _ := fragment.AddNode(distributeCertsNode)
		distributeKubeconfigID, _ := fragment.AddNode(distributeKubeconfigNode)
		restartKubeletID, _ := fragment.AddNode(restartKubeletNode)
		verifyKubeletID, _ := fragment.AddNode(verifyKubeletNode)
		restartKubeProxyID, _ := fragment.AddNode(restartKubeProxyNode)
		verifyKubeProxyID, _ := fragment.AddNode(verifyKubeProxyNode)

		fragment.AddDependency(backupKubeletID, distributeCertID)
		fragment.AddDependency(backupKubeletID, distributeKubeconfigID)
		fragment.AddDependency(distributeCertID, restartKubeletID)
		fragment.AddDependency(distributeKubeconfigID, restartKubeletID)
		fragment.AddDependency(distributeCertID, restartKubeProxyID)
		fragment.AddDependency(distributeKubeconfigID, restartKubeProxyID)
		fragment.AddDependency(restartKubeletID, verifyKubeletID)
		fragment.AddDependency(restartKubeProxyID, verifyKubeProxyID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, backupKubeletID)
		}

		barrierStep := stepcommon.NewNoOpStepBuilder(
			fmt.Sprintf("BarrierFor%s", hostName),
			fmt.Sprintf("Wait for all checks on node %s to complete", hostName),
		).Build()
		barrierNode := &plan.ExecutionNode{Name: fmt.Sprintf("WorkerUpdateBarrier_%s", hostName), Step: barrierStep, Hosts: hostList}
		barrierID, _ := fragment.AddNode(barrierNode)

		fragment.AddDependency(verifyKubeletID, barrierID)
		fragment.AddDependency(verifyKubeProxyID, barrierID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, backupKubeletID)
		}

		lastNodeExitPoint = barrierID
	}

	if len(masterHosts) > 0 {
		firstMaster := masterHosts[0]
		verifyClusterStep := health.NewCheckClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealthAfterWorkerRollout").Build()
		verifyClusterNode := &plan.ExecutionNode{
			Name:  "VerifyClusterHealthAfterWorkerRollout",
			Step:  verifyClusterStep,
			Hosts: []connector.Host{firstMaster},
		}
		verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
		}
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*UpdateWorkerNodesTask)(nil)
