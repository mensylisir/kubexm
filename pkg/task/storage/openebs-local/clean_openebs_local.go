package openebs_local

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	openebsstep "github.com/mensylisir/kubexm/pkg/step/storage/openebs-local"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanOpenebsTask struct {
	task.Base
}

func NewCleanOpenebsTask() task.Task {
	return &CleanOpenebsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanOpenebs",
				Description: "Clean up OpenEBS using Helm",
			},
		},
	}
}

func (t *CleanOpenebsTask) Name() string {
	return t.Meta.Name
}

func (t *CleanOpenebsTask) Description() string {
	return t.Meta.Description
}

func (t *CleanOpenebsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.OpenEBS == nil || cfg.Spec.Storage.OpenEBS.Enabled == nil {
		return false, nil
	}
	return *cfg.Spec.Storage.OpenEBS.Enabled, nil
}

func (t *CleanOpenebsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep := openebsstep.NewCleanOpenEBSStepBuilder(*runtimeCtx, "UninstallOpenEBSRelease").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallOpenEBSRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
