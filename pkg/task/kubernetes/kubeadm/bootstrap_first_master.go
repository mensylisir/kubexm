package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeconfig"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to bootstrap the cluster")
	}
	firstMasterHost := masterHosts[0]

	generateInitConfig := kubeadm.NewGenerateInitConfigStepBuilder(*runtimeCtx, "GenerateInitConfig").Build()

	kubeadmInit := kubeadm.NewKubeadmInitStepBuilder(*runtimeCtx, "KubeadmInit").Build()
	copyKubeconfig := kubeconfig.NewCopyKubeconfigStepBuilder(*runtimeCtx, "CopyAdminKubeconfig").Build()

	nodeGenerateConfig := &plan.ExecutionNode{
		Name:  "GenerateInitConfig",
		Step:  generateInitConfig,
		Hosts: []connector.Host{firstMasterHost},
	}
	if _, err := fragment.AddNode(nodeGenerateConfig); err != nil {
		return nil, err
	}

	nodeKubeadmInit := &plan.ExecutionNode{
		Name:  "KubeadmInit",
		Step:  kubeadmInit,
		Hosts: []connector.Host{firstMasterHost},
	}
	if _, err := fragment.AddNode(nodeKubeadmInit); err != nil {
		return nil, err
	}

	nodeCopyKubeconfig := &plan.ExecutionNode{
		Name:  "CopyAdminKubeconfig",
		Step:  copyKubeconfig,
		Hosts: []connector.Host{firstMasterHost},
	}
	if _, err := fragment.AddNode(nodeCopyKubeconfig); err != nil {
		return nil, err
	}

	if err := fragment.AddDependency("GenerateInitConfig", "KubeadmInit"); err != nil {
		return nil, err
	}

	if err := fragment.AddDependency("KubeadmInit", "CopyAdminKubeconfig"); err != nil {
		return nil, err
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
