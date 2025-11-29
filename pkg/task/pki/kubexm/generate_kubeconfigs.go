package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
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

func (t *GenerateKubeconfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	moveAssetsStep := kubexmstep.NewMoveNewAssetsStepBuilder(*runtimeCtx, "ActivateNewCertificates").Build()

	createKubeconfigsStep := kubexmstep.NewBinaryRenewAllKubeconfigsStepBuilder(*runtimeCtx, "GenerateAllKubeconfigs").Build()

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
