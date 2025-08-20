package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeadmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateKubeconfigsTask struct {
	task.Base
}

func NewGenerateKubeconfigsTask() task.Task {
	return &GenerateKubeconfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateKubeconfigs",
				Description: "Activates new certificates and generates all kubeconfig files based on them",
			},
		},
	}
}

func (t *GenerateKubeconfigsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateKubeconfigsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateKubeconfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if val, ok := ctx.GetModuleCache().Get(kubeadmstep.CacheKeyK8sCARequiresRenewal); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	if val, ok := ctx.GetModuleCache().Get(kubeadmstep.CacheKeyK8sLeafCertsRequireRenewal); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	return false, nil
}

func (t *GenerateKubeconfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	moveAssetsStep := kubeadmstep.NewKubeadmMoveNewAssetsStepBuilder(*runtimeCtx, "ActivateNewCertificates").Build()
	createKubeconfigsStep := kubeadmstep.NewKubeadmCreateKubeconfigsStepBuilder(*runtimeCtx, "CreateKubeconfigs").Build()

	moveAssetsNode := &plan.ExecutionNode{Name: "ActivateNewCertificates", Step: moveAssetsStep, Hosts: controlNodeList}
	createKubeconfigsNode := &plan.ExecutionNode{Name: "CreateAllKubeconfigFiles", Step: createKubeconfigsStep, Hosts: controlNodeList}

	moveAssetsNodeID, _ := fragment.AddNode(moveAssetsNode)
	createKubeconfigsNodeID, _ := fragment.AddNode(createKubeconfigsNode)
	fragment.AddDependency(moveAssetsNodeID, createKubeconfigsNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*GenerateKubeconfigsTask)(nil)
