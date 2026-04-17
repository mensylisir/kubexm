package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubexmstep "github.com/mensylisir/kubexm/internal/step/pki/kubexm"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanupTask struct {
	task.Base
}

func NewCleanupTask() task.Task {
	return &CleanupTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanupAfterRenewal",
				Description: "Cleans up temporary directories from the local workspace and remote nodes",
			},
		},
	}
}

func (t *CleanupTask) Name() string {
	return t.Meta.Name
}

func (t *CleanupTask) Description() string {
	return t.Meta.Description
}

func (t *CleanupTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Cleanup always runs after renewal to remove temporary assets.
	return true, nil
}

func (t *CleanupTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found for task %s", t.Name())
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	localCleanupStep, err := kubexmstep.NewLocalCleanupStepBuilder(runtimeCtx, "CleanupLocalWorkspace").Build()
	if err != nil {
		return nil, err
	}
	remoteCleanupStep, err := kubexmstep.NewRemoteCleanupStepBuilder(runtimeCtx, "CleanupRemoteBackups").Build()
	if err != nil {
		return nil, err
	}

	localCleanupNode := &plan.ExecutionNode{
		Name:  "CleanupLocalWorkspace",
		Step:  localCleanupStep,
		Hosts: []remotefw.Host{controlNode},
	}

	remoteCleanupNode := &plan.ExecutionNode{
		Name:  "CleanupRemoteBackups",
		Step:  remoteCleanupStep,
		Hosts: allHosts,
	}

	fragment.AddNode(localCleanupNode)
	fragment.AddNode(remoteCleanupNode)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CleanupTask)(nil)
