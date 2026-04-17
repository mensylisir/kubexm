package kubeadm

import (
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	cleanupStep, err := kubeadm.NewKubeadmLocalCleanupStepBuilder(runtimeCtx, "CleanupLocalWorkspace").Build()
	if err != nil {
		return nil, err
	}
	cleanupNode := &plan.ExecutionNode{Name: "CleanupLocalWorkspace", Step: cleanupStep, Hosts: []remotefw.Host{controlNode}}

	fragment.AddNode(cleanupNode, "CleanupLocalWorkspace")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
