package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeadmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateK8sLeafCertsTask struct {
	task.Base
}

func NewGenerateK8sLeafCertsTask() task.Task {
	return &GenerateK8sLeafCertsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateK8sLeafCerts",
				Description: "Generates new K8s leaf certificates using the existing CA",
			},
		},
	}
}

func (t *GenerateK8sLeafCertsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateK8sLeafCertsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateK8sLeafCertsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	rawCaRenewal, ok := ctx.GetModuleCache().Get(kubeadmstep.CacheKeyK8sCARequiresRenewal)
	if !ok {
		rawCaRenewal = false
	}

	caRequiresRenewal, isBool := rawCaRenewal.(bool)
	if !isBool {
		caRequiresRenewal = false
	}

	rawLeafRenewal, ok := ctx.GetModuleCache().Get(kubeadmstep.CacheKeyK8sLeafCertsRequireRenewal)
	if !ok {
		rawLeafRenewal = false
	}

	leafRequiresRenewal, isBool := rawLeafRenewal.(bool)
	if !isBool {
		leafRequiresRenewal = false
	}

	if !caRequiresRenewal && leafRequiresRenewal {
		return true, nil
	}

	return false, nil
}

func (t *GenerateK8sLeafCertsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext's base context is not of type *runtime.Context")
	}

	renewLeafStep := kubeadmstep.NewKubeadmRenewLeafCertsStepBuilder(*runtimeCtx, "RenewK8sLeafCerts").Build()

	renewLeafNode := &plan.ExecutionNode{
		Name: "RenewK8sLeafCerts",
		Step: renewLeafStep,
	}

	renewLeafNodeID, _ := fragment.AddNode(renewLeafNode)
	fragment.EntryNodes = []plan.NodeID{renewLeafNodeID}
	fragment.ExitNodes = []plan.NodeID{renewLeafNodeID}

	return fragment, nil
}

var _ task.Task = (*GenerateK8sLeafCertsTask)(nil)
