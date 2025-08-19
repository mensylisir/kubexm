package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanupWorkspaceTask struct {
	task.Base
}

func NewCleanupWorkspaceTask() task.Task {
	return &CleanupWorkspaceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanupLocalWorkspace",
				Description: "Cleans up temporary directories (e.g., certs-new, certs-old) from the local workspace",
			},
		},
	}
}

func (t *CleanupWorkspaceTask) Name() string {
	return t.Meta.Name
}

func (t *CleanupWorkspaceTask) Description() string {
	return t.Meta.Description
}

func (t *CleanupWorkspaceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanupWorkspaceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	cleanupStep := kubeadm.NewKubeadmLocalCleanupStepBuilder(*runtimeCtx, "CleanupLocalWorkspace").Build()
	cleanupNode := &plan.ExecutionNode{Name: "CleanupLocalWorkspace", Step: cleanupStep, Hosts: []connector.Host{controlNode}}

	fragment.AddNode(cleanupNode, "CleanupLocalWorkspace")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
