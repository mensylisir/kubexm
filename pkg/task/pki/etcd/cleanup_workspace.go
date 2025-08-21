package etcd

import (
	"github.com/mensylisir/kubexm/pkg/common"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/pki/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type FinalizeWorkspaceTask struct {
	task.Base
}

func NewFinalizeWorkspaceTask() task.Task {
	return &FinalizeWorkspaceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "FinalizeWorkspace",
				Description: "Finalizes the CA rotation by moving new assets to the main directory and cleaning up the workspace",
			},
		},
	}
}

func (t *FinalizeWorkspaceTask) Name() string {
	return t.Meta.Name
}

func (t *FinalizeWorkspaceTask) Description() string {
	return t.Meta.Description
}

func (t *FinalizeWorkspaceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	caRenewVal, _ := ctx.GetModuleCache().Get(common.CacheKubexmEtcdCACertRenew)
	caRenew, _ := caRenewVal.(bool)

	return caRenew, nil
}

func (t *FinalizeWorkspaceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	cleanupStep := etcdstep.NewCleanupTransitionAssetsStepBuilder(*runtimeCtx, "CleanupTransitionAssets").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanupWorkspace", Step: cleanupStep, Hosts: []connector.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*FinalizeWorkspaceTask)(nil)
