package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"

	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeadmstep "github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

type GenerateK8sCACertsTask struct {
	task.Base
}

func NewGenerateK8sCACertsTask() task.Task {
	return &GenerateK8sCACertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateK8sCACerts",
				Description: "Generates a new K8s CA and corresponding new leaf certificates",
			},
		},
	}
}

func (t *GenerateK8sCACertsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateK8sCACertsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateK8sCACertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	var caRequiresRenewal bool
	runtimeCtx := ctx.(*runtime.Context)
	cacheKey := fmt.Sprintf(common.CacheKubeadmK8sCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	if rawVal, ok := ctx.GetModuleCache().Get(cacheKey); ok {
		if val, isBool := rawVal.(bool); isBool {
			caRequiresRenewal = val
		}
	}

	return caRequiresRenewal, nil
}

func (t *GenerateK8sCACertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	prepareAssetsStep, err := kubeadmstep.NewKubeadmPrepareCAAssetsStepBuilder(runtimeCtx, "PrepareK8sCAAssets").Build()
	if err != nil {
		return nil, err
	}
	renewCaStep, err := kubeadmstep.NewKubeadmRenewK8sCAStepBuilder(runtimeCtx, "RenewK8sCA").Build()
	if err != nil {
		return nil, err
	}
	renewLeafStep, err := kubeadmstep.NewKubeadmRenewLeafCertsStepBuilder(runtimeCtx, "RenewK8sLeafCerts").Build()
	if err != nil {
		return nil, err
	}
	generateCaBundle, err := kubeadmstep.NewKubeadmPrepareCATransitionStepBuilder(runtimeCtx, "GenerateK8sCABundle").Build()
	if err != nil {
		return nil, err
	}

	prepareAssetsNode := &plan.ExecutionNode{Name: "PrepareK8sCAAssets", Step: prepareAssetsStep}
	renewCaNode := &plan.ExecutionNode{Name: "RenewK8sCA", Step: renewCaStep}
	renewLeafNode := &plan.ExecutionNode{Name: "RenewK8sLeafCerts", Step: renewLeafStep}
	generateBundleNode := &plan.ExecutionNode{Name: "GenerateK8sCABundle", Step: generateCaBundle}

	prepareAssetsNodeID, _ := fragment.AddNode(prepareAssetsNode)
	renewCaNodeID, _ := fragment.AddNode(renewCaNode)
	renewLeafNodeID, _ := fragment.AddNode(renewLeafNode)
	generateBundleNodeID, _ := fragment.AddNode(generateBundleNode)

	fragment.AddDependency(prepareAssetsNodeID, renewCaNodeID)
	fragment.AddDependency(renewCaNodeID, renewLeafNodeID)
	fragment.AddDependency(renewCaNodeID, generateBundleNodeID)
	fragment.AddDependency(prepareAssetsNodeID, generateBundleNodeID)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*GenerateK8sCACertsTask)(nil)
