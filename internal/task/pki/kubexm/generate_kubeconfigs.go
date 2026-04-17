package kubexm

import (
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubexmstep "github.com/mensylisir/kubexm/internal/step/pki/kubexm"
	"github.com/mensylisir/kubexm/internal/task"
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
	// Renewal is user-initiated; always plan this task when PKIModule is invoked.
	return true, nil
}

func (t *GenerateKubeconfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []remotefw.Host{controlNode}

	moveAssetsStep, err := kubexmstep.NewMoveNewAssetsStepBuilder(runtimeCtx, "ActivateNewCertificates").Build()
	if err != nil {
		return nil, err
	}

	createKubeconfigsStep, err := kubexmstep.NewBinaryRenewAllKubeconfigsStepBuilder(runtimeCtx, "GenerateAllKubeconfigs").Build()
	if err != nil {
		return nil, err
	}

	moveAssetsNode := &plan.ExecutionNode{
		Name:  "ActivateNewCertificates",
		Step:  moveAssetsStep,
		Hosts: controlNodeList,
	}
	createKubeconfigsNode := &plan.ExecutionNode{
		Name:  "CreateAllKubeconfigFiles",
		Step:  createKubeconfigsStep,
		Hosts: controlNodeList,
	}

	moveAssetsNodeID, _ := fragment.AddNode(moveAssetsNode)
	createKubeconfigsNodeID, _ := fragment.AddNode(createKubeconfigsNode)

	fragment.AddDependency(moveAssetsNodeID, createKubeconfigsNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*GenerateKubeconfigsTask)(nil)
