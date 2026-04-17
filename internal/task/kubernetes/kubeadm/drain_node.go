package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/perform"
	"github.com/mensylisir/kubexm/internal/task"
)

type DrainNodeTask struct {
	task.Base
}

func NewDrainNodeTask() task.Task {
	return &DrainNodeTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DrainNode",
				Description: "Drains worker nodes before removal from cluster",
			},
		},
	}
}

func (t *DrainNodeTask) Name() string        { return t.Meta.Name }
func (t *DrainNodeTask) Description() string { return t.Meta.Description }

func (t *DrainNodeTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *DrainNodeTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		ctx.GetLogger().Info("No worker hosts found, skipping drain")
		return fragment, nil
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node: %w", err)
	}

	for _, worker := range workerHosts {
		drainStep, err := perform.NewDrainNodeStepBuilder(runtimeCtx, fmt.Sprintf("Drain%s", worker.GetName()), worker.GetName()).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create drain step for %s: %w", worker.GetName(), err)
		}
		fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("Drain%s", worker.GetName()),
			Step:  drainStep,
			Hosts: []remotefw.Host{controlNode},
		})
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DrainNodeTask)(nil)
