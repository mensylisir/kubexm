package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	kubeadmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
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
				Description: "Distributes new CA and kubelet.conf to all worker nodes and performs a rolling restart of kubelet",
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
	caCacheKey := fmt.Sprintf(common.CacheKubeadmK8sCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	if val, ok := ctx.GetModuleCache().Get(caCacheKey); ok {
		if renew, isBool := val.(bool); isBool && renew {
			renewalTriggered = true
		}
	}
	if !renewalTriggered {
		leafCacheKey := fmt.Sprintf(common.CacheKubeadmK8sLeafCertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
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
		ctx.GetLogger().Info("No pure worker nodes found to update. Skipping this task's planning.")
		return fragment, nil
	}

	ctx.GetLogger().Infof("Planning update for %d pure worker nodes.", len(pureWorkerHosts))

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range pureWorkerHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		distributeCAStep := kubeadmstep.NewKubeadmDistributeK8sPKIStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeCAFor%s", hostName)).Build()
		distributeKubeletConfStep := kubeadmstep.NewKubeadmDistributeKubeletConfigStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeKubeletConfFor%s", hostName)).Build()
		restartKubeletStep := kubeadmstep.NewKubeadmRestartKubeletStepBuilder(*runtimeCtx, fmt.Sprintf("RestartKubeletFor%s", hostName)).Build()
		verifyWorkerStep := kubeadm.NewKubeadmVerifyWorkerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyWorkerFor%s", hostName)).Build()

		distributeCANode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeCAFor%s", hostName), Step: distributeCAStep, Hosts: hostList}
		distributeKubeletConfNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeKubeletConfFor%s", hostName), Step: distributeKubeletConfStep, Hosts: hostList}
		restartKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartKubeletFor%s", hostName), Step: restartKubeletStep, Hosts: hostList}
		verifyWorkerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyWorkerFor%s", hostName), Step: verifyWorkerStep, Hosts: hostList}

		distributeCAID, _ := fragment.AddNode(distributeCANode)
		distributeKubeletConfID, _ := fragment.AddNode(distributeKubeletConfNode)
		restartKubeletID, _ := fragment.AddNode(restartKubeletNode)
		verifyWorkerID, _ := fragment.AddNode(verifyWorkerNode)

		fragment.AddDependency(distributeCAID, distributeKubeletConfID)
		fragment.AddDependency(distributeKubeletConfID, restartKubeletID)
		fragment.AddDependency(restartKubeletID, verifyWorkerID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, distributeCAID)
		}
		lastNodeExitPoint = verifyWorkerID
	}

	firstMaster := masterHosts[0]
	verifyClusterStep := kubeadm.NewKubeadmVerifyClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealthAfterWorkerRollout").Build()
	verifyClusterNode := &plan.ExecutionNode{
		Name:  "VerifyClusterHealthAfterWorkerRollout",
		Step:  verifyClusterStep,
		Hosts: []connector.Host{firstMaster},
	}
	verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

	if lastNodeExitPoint != "" {
		fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*UpdateWorkerNodesTask)(nil)
