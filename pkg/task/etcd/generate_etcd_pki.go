package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdcertsstep "github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateEtcdPKITask struct {
	task.Base
}

func NewGenerateEtcdPKITask() task.Task {
	return &GenerateEtcdPKITask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateEtcdPKI",
				Description: "Generate etcd CA and certificates for all etcd nodes",
			},
		},
	}
}

func (t *GenerateEtcdPKITask) Name() string                                     { return t.Meta.Name }
func (t *GenerateEtcdPKITask) Description() string                              { return t.Meta.Description }
func (t *GenerateEtcdPKITask) IsRequired(ctx runtime.TaskContext) (bool, error) { return true, nil }

func (t *GenerateEtcdPKITask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHost := []connector.Host{controlNode}

	generateEtcdCA := etcdcertsstep.NewGenerateEtcdCAStepBuilder(*runtimeCtx, "GenerateEtcdCA").Build()
	generateEtcdCerts := etcdcertsstep.NewGenerateEtcdCertsStepBuilder(*runtimeCtx, "GenerateEtcdCerts").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateEtcdCA", Step: generateEtcdCA, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateEtcdCerts", Step: generateEtcdCerts, Hosts: executionHost})

	fragment.AddDependency("GenerateEtcdCA", "GenerateEtcdCerts")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
