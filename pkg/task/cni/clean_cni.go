package cni

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/cni"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanCNIPluginsTask struct {
	task.Base
}

func NewCleanCNIPluginsTask() task.Task {
	return &CleanCNIPluginsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanCNIPlugins",
				Description: "Remove CNI plugin binaries and configuration files from all nodes",
			},
		},
	}
}

func (t *CleanCNIPluginsTask) Name() string {
	return t.Meta.Name
}

func (t *CleanCNIPluginsTask) Description() string {
	return t.Meta.Description
}

func (t *CleanCNIPluginsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanCNIPluginsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		ctx.GetLogger().Info("No master or worker hosts found, skipping CNI cleanup task.")
		return fragment, nil
	}

	cleanCniStep := cni.NewCleanCNIStepBuilder(*runtimeCtx, "RemoveCNIDirectories").Build()

	node := &plan.ExecutionNode{
		Name:  "RemoveCNIDirectories",
		Step:  cleanCniStep,
		Hosts: deployHosts,
	}
	if _, err := fragment.AddNode(node); err != nil {
		return nil, err
	}
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
