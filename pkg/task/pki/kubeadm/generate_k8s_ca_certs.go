package kubeadm

import (
	"github.com/mensylisir/kubexm/pkg/common"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeadmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
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

	if rawVal, ok := ctx.GetModuleCache().Get(common.CacheKubeadmK8sCACertRenew); ok {
		if val, isBool := rawVal.(bool); isBool {
			caRequiresRenewal = val
		}
	}

	return caRequiresRenewal, nil
}

func (t *GenerateK8sCACertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	prepareAssetsStep := kubeadmstep.NewKubeadmPrepareCAAssetsStepBuilder(*runtimeCtx, "PrepareK8sCAAssets").Build()
	renewCaStep := kubeadmstep.NewKubeadmRenewK8sCAStepBuilder(*runtimeCtx, "RenewK8sCA").Build()
	renewLeafStep := kubeadmstep.NewKubeadmRenewLeafCertsStepBuilder(*runtimeCtx, "RenewK8sLeafCerts").Build()
	generateCaBundle := kubeadmstep.NewKubeadmPrepareCATransitionStepBuilder(*runtimeCtx, "GenerateK8sCABundle").Build()

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
