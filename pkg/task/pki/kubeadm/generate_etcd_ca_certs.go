package kubeadm

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateEtcdCABundleCertsTask struct {
	task.Base
}

func NewGenerateEtcdCABundleCertsTask() task.Task {
	return &GenerateEtcdCABundleCertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateEtcdCACerts",
				Description: "Generates new Etcd CA, creates CA bundle, and generates new leaf certificates",
			},
		},
	}
}

func (t *GenerateEtcdCABundleCertsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateEtcdCABundleCertsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateEtcdCABundleCertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		return true, nil
	}
	return false, nil
}

func (t *GenerateEtcdCABundleCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	prepareCAAssetsStep := kubeadm.NewKubeadmPrepareStackedEtcdCAAssetsStepBuilder(*runtimeCtx, "PrepareEtcdCAAssets").Build()
	renewCAStep := kubeadm.NewKubeadmRenewStackedEtcdCAStepBuilder(*runtimeCtx, "RenewEtcdCA").Build()
	renewLeafsStep := kubeadm.NewKubeadmRenewStackedEtcdLeafCertsStepBuilder(*runtimeCtx, "RenewEtcdLeafs").Build()
	createCABundleStep := kubeadm.NewKubeadmPrepareStackedEtcdCATransitionStepBuilder(*runtimeCtx, "CreateEtcdCABundle").Build()

	prepareCAAssetsNode := &plan.ExecutionNode{Name: "PrepareEtcdCAAssets", Step: prepareCAAssetsStep, Hosts: controlNodeList}
	renewCANode := &plan.ExecutionNode{Name: "RenewEtcdCA", Step: renewCAStep, Hosts: controlNodeList}
	renewLeafsNode := &plan.ExecutionNode{Name: "RenewEtcdLeafs", Step: renewLeafsStep, Hosts: controlNodeList}
	createCABundleNode := &plan.ExecutionNode{Name: "CreateEtcdCABundle", Step: createCABundleStep, Hosts: controlNodeList}

	fragment.AddNode(prepareCAAssetsNode, "PrepareEtcdCAAssets")
	fragment.AddNode(renewCANode, "RenewEtcdCA")
	fragment.AddNode(renewLeafsNode, "RenewEtcdLeafs")
	fragment.AddNode(createCABundleNode, "CreateEtcdCABundle")

	fragment.AddDependency("PrepareEtcdCAAssets", "RenewEtcdCA")
	fragment.AddDependency("RenewEtcdCA", "RenewEtcdLeafs")
	fragment.AddDependency("RenewEtcdCA", "CreateEtcdCABundle")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
