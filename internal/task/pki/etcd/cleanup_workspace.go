package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"

	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	etcdstep "github.com/mensylisir/kubexm/internal/step/pki/etcd"
	"github.com/mensylisir/kubexm/internal/task"
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
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return false, fmt.Errorf("ctx is not *runtime.Context")
	}
	cacheKey := fmt.Sprintf(common.CacheKubexmEtcdCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	caRenewVal, _ := ctx.GetModuleCache().Get(cacheKey)
	caRenew, _ := caRenewVal.(bool)

	return caRenew, nil
}

func (t *FinalizeWorkspaceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx := ctx.ForTask(t.Name())

	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	cleanupStep, err := etcdstep.NewCleanupTransitionAssetsStepBuilder(runtimeCtx, "CleanupTransitionAssets").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanupWorkspace", Step: cleanupStep, Hosts: []remotefw.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*FinalizeWorkspaceTask)(nil)
