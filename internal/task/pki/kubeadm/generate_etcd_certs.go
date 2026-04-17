package kubeadm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
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
	etcdSpec := ctx.GetClusterConfig().Spec.Etcd
	if etcdSpec == nil {
		return false, nil
	}
	if etcdSpec.Type == string(common.EtcdDeploymentTypeKubeadm) {
		return true, nil
	}
	return false, nil
}

func (t *GenerateLeafCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []remotefw.Host{controlNode}

	renewLeafsStep, err := kubeadm.NewKubeadmRenewStackedEtcdLeafCertsStepBuilder(runtimeCtx, "RenewEtcdLeafs").Build()
	if err != nil {
		return nil, err
	}

	renewLeafsNode := &plan.ExecutionNode{Name: "RenewEtcdLeafs", Step: renewLeafsStep, Hosts: controlNodeList}

	fragment.AddNode(renewLeafsNode, "RenewEtcdLeafs")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
