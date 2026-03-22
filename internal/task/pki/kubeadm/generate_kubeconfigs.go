package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"

	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeadmstep "github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
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
	runtimeCtx := ctx.(*runtime.Context)
	caCacheKey := fmt.Sprintf(common.CacheKubeadmK8sCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	if val, ok := ctx.GetModuleCache().Get(caCacheKey); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	leafCacheKey := fmt.Sprintf(common.CacheKubeadmK8sLeafCertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	if val, ok := ctx.GetModuleCache().Get(leafCacheKey); ok {
		if renew, isBool := val.(bool); isBool && renew {
			return true, nil
		}
	}
	return false, nil
}

func (t *GenerateKubeconfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	moveAssetsStep, err := kubeadmstep.NewKubeadmMoveNewAssetsStepBuilder(runtimeCtx, "ActivateNewCertificates").Build()
	if err != nil {
		return nil, err
	}
	createKubeconfigsStep, err := kubeadmstep.NewKubeadmCreateKubeconfigsStepBuilder(runtimeCtx, "CreateKubeconfigs").Build()
	if err != nil {
		return nil, err
	}

	moveAssetsNode := &plan.ExecutionNode{Name: "ActivateNewCertificates", Step: moveAssetsStep, Hosts: controlNodeList}
	createKubeconfigsNode := &plan.ExecutionNode{Name: "CreateAllKubeconfigFiles", Step: createKubeconfigsStep, Hosts: controlNodeList}

	moveAssetsNodeID, _ := fragment.AddNode(moveAssetsNode)
	createKubeconfigsNodeID, _ := fragment.AddNode(createKubeconfigsNode)
	fragment.AddDependency(moveAssetsNodeID, createKubeconfigsNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*GenerateKubeconfigsTask)(nil)
