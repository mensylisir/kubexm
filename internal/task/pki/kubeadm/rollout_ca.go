package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeadmstep "github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
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
				Description: "Distributes all final kubeconfigs to masters and restarts all necessary components",
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
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return false, fmt.Errorf("ctx is not *runtime.Context")
	}
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

func (t *FinalizeMastersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found for task %s", t.Name())
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range masterHosts {
		hostName := host.GetName()
		hostList := []remotefw.Host{host}

		distributeKubeconfigsStep, err := kubeadmstep.NewKubeadmDistributeKubeconfigsStepBuilder(runtimeCtx, fmt.Sprintf("DistributeKubeconfigsFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		distributeKubeletConfStep, err := kubeadmstep.NewKubeadmDistributeKubeletConfigStepBuilder(runtimeCtx, fmt.Sprintf("DistributeKubeletConfFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		distributeCA, err := kubeadmstep.NewKubeadmDistributeK8sPKIStepBuilder(runtimeCtx, fmt.Sprintf("DistributeCAFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		restartKubeletStep, err := kubeadmstep.NewKubeadmRestartKubeletStepBuilder(runtimeCtx, fmt.Sprintf("RestartKubeletFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifyKubeletStep, err := kubeadm.NewKubeadmVerifyWorkerHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifyKubeletFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		restartControlPlaneStep, err := kubeadmstep.NewKubeadmRestartControlPlaneStepBuilder(runtimeCtx, fmt.Sprintf("RestartCPForFinal%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		verifyControlPlaneStep, err := kubeadm.NewKubeadmVerifyControlPlaneHealthStepBuilder(runtimeCtx, fmt.Sprintf("VerifyCPForFinalFor%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		distributeKubeconfigsNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeKubeconfigsFor%s", hostName), Step: distributeKubeconfigsStep, Hosts: hostList}
		distributeKubeletConfNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeKubeletConfFor%s", hostName), Step: distributeKubeletConfStep, Hosts: hostList}
		distributeCANode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeCAFor%s", hostName), Step: distributeCA, Hosts: hostList}

		restartKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartKubeletFor%s", hostName), Step: restartKubeletStep, Hosts: hostList}
		verifyKubeletNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyKubeletFor%s", hostName), Step: verifyKubeletStep, Hosts: hostList}

		restartCPNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartCP_FinalFor%s", hostName), Step: restartControlPlaneStep, Hosts: hostList}
		verifyCPNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyCP_FinalFor%s", hostName), Step: verifyControlPlaneStep, Hosts: hostList}

		distributeKubeconfigsID, _ := fragment.AddNode(distributeKubeconfigsNode)
		distributeKubeletConfID, _ := fragment.AddNode(distributeKubeletConfNode)
		distributeCAID, _ := fragment.AddNode(distributeCANode)
		restartKubeletID, _ := fragment.AddNode(restartKubeletNode)
		verifyKubeletID, _ := fragment.AddNode(verifyKubeletNode)
		restartCPID, _ := fragment.AddNode(restartCPNode)
		verifyCPID, _ := fragment.AddNode(verifyCPNode)

		fragment.AddDependency(distributeKubeconfigsID, restartCPID)
		fragment.AddDependency(distributeKubeletConfID, restartKubeletID)
		fragment.AddDependency(distributeCAID, restartKubeletID)
		fragment.AddDependency(distributeCAID, restartCPID)

		fragment.AddDependency(restartKubeletID, verifyKubeletID)
		fragment.AddDependency(verifyKubeletID, restartCPID)
		fragment.AddDependency(restartCPID, verifyCPID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, distributeKubeconfigsID)
			fragment.AddDependency(lastNodeExitPoint, distributeKubeletConfID)
			fragment.AddDependency(lastNodeExitPoint, distributeCAID)
		}

		lastNodeExitPoint = verifyCPID
	}

	verifyClusterStep, err := kubeadm.NewKubeadmVerifyClusterHealthStepBuilder(runtimeCtx, "FinalClusterHealthVerification").Build()
	if err != nil {
		return nil, err
	}
	verifyClusterNode := &plan.ExecutionNode{
		Name:  "FinalClusterHealthVerification",
		Step:  verifyClusterStep,
		Hosts: []remotefw.Host{masterHosts[0]},
	}
	verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

	if lastNodeExitPoint != "" {
		fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*FinalizeMastersTask)(nil)
