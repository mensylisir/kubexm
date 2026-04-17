package longhorn

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/storage/longhorn"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanLonghornTask struct {
	task.Base
}

func NewCleanLonghornTask() task.Task {
	return &CleanLonghornTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanLonghorn",
				Description: "Uninstall Longhorn and cleanup all related data and resources",
			},
		},
	}
}

func (t *CleanLonghornTask) Name() string {
	return t.Meta.Name
}

func (t *CleanLonghornTask) Description() string {
	return t.Meta.Description
}

func (t *CleanLonghornTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Storage == nil || ctx.GetClusterConfig().Spec.Storage.Longhorn == nil {
		return false, nil
	}
	return true, nil
}

func (t *CleanLonghornTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(allHosts) == 0 {
		return fragment, nil
	}

	cleanStepBuilder := longhorn.NewCleanLonghornStepBuilder(runtimeCtx, "CleanLonghorn")
	cleanStep, err := cleanStepBuilder.WithPurgeData(true).Build()
	if err != nil {
		return nil, err
	}
	node := &plan.ExecutionNode{Name: "CleanLonghorn", Step: cleanStep, Hosts: allHosts}
	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
