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

type GenerateLeafCertsTask struct {
	task.Base
}

func NewGenerateLeafCertsTask() task.Task {
	return &GenerateLeafCertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateEtcdLeafCerts",
				Description: "Generates new Etcd leaf certificates using the existing CA in the local workspace",
			},
		},
	}
}

func (t *GenerateLeafCertsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateLeafCertsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateLeafCertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		return true, nil
	}
	return false, nil
}

func (t *GenerateLeafCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	renewLeafsStep := kubeadm.NewKubeadmRenewStackedEtcdLeafCertsStepBuilder(*runtimeCtx, "RenewEtcdLeafs").Build()

	renewLeafsNode := &plan.ExecutionNode{Name: "RenewEtcdLeafs", Step: renewLeafsStep, Hosts: controlNodeList}

	fragment.AddNode(renewLeafsNode, "RenewEtcdLeafs")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
