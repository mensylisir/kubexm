package kubexm

import (
	"github.com/mensylisir/kubexm/pkg/common"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateClusterCertsTask struct {
	task.Base
}

func NewGenerateClusterCertsTask() task.Task {
	return &GenerateClusterCertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateNewAssets",
				Description: "Generates new CA, leaf certificates, and the transition CA bundle in the local workspace",
			},
		},
	}
}

func (t *GenerateClusterCertsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateClusterCertsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateClusterCertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
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

	if !renewalTriggered {
		ctx.GetLogger().Info("Skipping asset generation task: No certificate renewal required.")
	}

	return renewalTriggered, nil
}

func (t *GenerateClusterCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	prepareAssetsStep := kubexmstep.NewKubexmPrepareCAAssetsStepBuilder(*runtimeCtx, "PrepareCAAssets").Build()
	renewCaStep := kubexmstep.NewKubexmRenewK8sCAStepBuilder(*runtimeCtx, "RenewK8sCA").Build()
	renewLeafsStep := kubexmstep.NewBinaryRenewAllLeafCertsStepBuilder(*runtimeCtx, "RenewAllLeafCerts").Build()
	createBundleStep := kubexmstep.NewKubexmPrepareCATransitionStepBuilder(*runtimeCtx, "CreateCABundle").Build()

	prepareAssetsNode := &plan.ExecutionNode{Name: "PrepareWorkspaceForRenewal", Step: prepareAssetsStep, Hosts: controlNodeList}
	renewCaNode := &plan.ExecutionNode{Name: "RenewCoreCAs", Step: renewCaStep, Hosts: controlNodeList}
	renewLeafsNode := &plan.ExecutionNode{Name: "RenewAllLeafCertificates", Step: renewLeafsStep, Hosts: controlNodeList}
	createBundleNode := &plan.ExecutionNode{Name: "CreateTransitionCABundle", Step: createBundleStep, Hosts: controlNodeList}

	prepareAssetsNodeID, _ := fragment.AddNode(prepareAssetsNode)
	renewCaNodeID, _ := fragment.AddNode(renewCaNode)
	renewLeafsNodeID, _ := fragment.AddNode(renewLeafsNode)
	createBundleNodeID, _ := fragment.AddNode(createBundleNode)

	fragment.AddDependency(prepareAssetsNodeID, renewCaNodeID)
	fragment.AddDependency(prepareAssetsNodeID, createBundleNodeID)

	fragment.AddDependency(renewCaNodeID, renewLeafsNodeID)
	fragment.AddDependency(renewCaNodeID, createBundleNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*GenerateClusterCertsTask)(nil)
