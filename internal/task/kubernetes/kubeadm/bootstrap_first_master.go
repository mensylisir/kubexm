package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeconfig"
	"github.com/mensylisir/kubexm/internal/task"
)

type BootstrapFirstMasterTask struct {
	task.Base
}

func NewBootstrapFirstMasterTask() task.Task {
	return &BootstrapFirstMasterTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "BootstrapFirstMaster",
				Description: "Initialize the first master node using kubeadm",
			},
		},
	}
}

func (t *BootstrapFirstMasterTask) Name() string {
	return t.Meta.Name
}

func (t *BootstrapFirstMasterTask) Description() string {
	return t.Meta.Description
}

func (t *BootstrapFirstMasterTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *BootstrapFirstMasterTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to bootstrap the cluster")
	}
	firstMasterHost := masterHosts[0]

	generateInitConfig, err := kubeadm.NewGenerateInitConfigStepBuilder(runtimeCtx, "GenerateInitConfig").Build()
	if err != nil {
		return nil, err
	}
	kubeadmInit, err := kubeadm.NewKubeadmInitStepBuilder(runtimeCtx, "KubeadmInit").Build()
	if err != nil {
		return nil, err
	}
	copyKubeconfig, err := kubeconfig.NewCopyKubeconfigStepBuilder(runtimeCtx, "CopyAdminKubeconfig").Build()
	if err != nil {
		return nil, err
	}

	nodeGenerateConfig := &plan.ExecutionNode{Name: "GenerateInitConfig", Step: generateInitConfig, Hosts: []connector.Host{firstMasterHost}}
	nodeKubeadmInit := &plan.ExecutionNode{Name: "KubeadmInit", Step: kubeadmInit, Hosts: []connector.Host{firstMasterHost}}
	nodeCopyKubeconfig := &plan.ExecutionNode{Name: "CopyAdminKubeconfig", Step: copyKubeconfig, Hosts: []connector.Host{firstMasterHost}}

	fragment.AddNode(nodeGenerateConfig)
	fragment.AddNode(nodeKubeadmInit)
	fragment.AddNode(nodeCopyKubeconfig)

	fragment.AddDependency("GenerateInitConfig", "KubeadmInit")
	fragment.AddDependency("KubeadmInit", "CopyAdminKubeconfig")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
