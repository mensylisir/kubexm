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

type PrepareRenewalWorkspaceTask struct {
	task.Base
}

func NewPrepareRenewalWorkspaceTask() task.Task {
	return &PrepareRenewalWorkspaceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PrepareRenewalWorkspace",
				Description: "Prepares a secure workspace on the control node for CA rotation by backing up the current CA",
			},
		},
	}
}

func (t *PrepareRenewalWorkspaceTask) Name() string {
	return t.Meta.Name
}

func (t *PrepareRenewalWorkspaceTask) Description() string {
	return t.Meta.Description
}

func (t *PrepareRenewalWorkspaceTask) GetBase() *task.Base {
	return &t.Base
}

func (t *PrepareRenewalWorkspaceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	requiresRenewal, found := ctx.GetPipelineCache().Get(etcdstep.CacheKeyCARequiresRenewal)
	if !found {
		return false, nil
	}

	if renew, ok := requiresRenewal.(bool); ok {
		return renew, nil
	}

	return false, fmt.Errorf("invalid type for cache key '%s': expected bool, got %T", etcdstep.CacheKeyCARequiresRenewal, requiresRenewal)
}

func (t *PrepareRenewalWorkspaceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context in Plan method")
	}

	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for task %s: %w", t.Name(), err)
	}

	prepareAssetsStep := etcdstep.NewPrepareAssetsStepBuilder(*runtimeCtx, "PrepareEtcdCARenewalAssets").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareAssets", Step: prepareAssetsStep, Hosts: []connector.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*PrepareRenewalWorkspaceTask)(nil)
