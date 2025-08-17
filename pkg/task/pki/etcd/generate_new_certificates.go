package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/pki/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateNewCertificatesTask struct {
	task.Base
}

func NewGenerateNewCertificatesTask() task.Task {
	return &GenerateNewCertificatesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateNewCertificates",
				Description: "Generates new ETCD CA and/or leaf certificates on the control node",
			},
		},
	}
}

func (t *GenerateNewCertificatesTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateNewCertificatesTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateNewCertificatesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	caRenewVal, _ := ctx.GetModuleCache().Get(etcdstep.CacheKeyCARequiresRenewal)
	caRenew, _ := caRenewVal.(bool)
	leafRenewVal, _ := ctx.GetModuleCache().Get(etcdstep.CacheKeyLeafRequiresRenewal)
	leafRenew, _ := leafRenewVal.(bool)

	return caRenew || leafRenew, nil
}

func (t *GenerateNewCertificatesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context in Plan method")
	}
	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for task %s: %w", t.Name(), err)
	}

	resignCaStep := etcdstep.NewResignCAStepBuilder(*runtimeCtx, "ResignEtcdCA").Build()
	genLeafsStep := etcdstep.NewGenerateNewLeafCertsStepBuilder(*runtimeCtx, "GenerateNewEtcdLeafCerts").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "ResignEtcdCA", Step: resignCaStep, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNewEtcdLeafCerts", Step: genLeafsStep, Hosts: []connector.Host{controlNode}})
	fragment.AddDependency("ResignEtcdCA", "GenerateNewEtcdLeafCerts")
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*GenerateNewCertificatesTask)(nil)
