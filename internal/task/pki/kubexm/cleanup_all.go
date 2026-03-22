package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"

	"github.com/mensylisir/kubexm/internal/connector"
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
	var renewalTriggered bool
	runtimeCtx := ctx.(*runtime.Context)
	caCacheKey := fmt.Sprintf(common.CacheKubexmK8sCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	if val, ok := ctx.GetModuleCache().Get(caCacheKey); ok {
		if renew, isBool := val.(bool); isBool && renew {
			renewalTriggered = true
		}
	}
	if !renewalTriggered {
		leafCacheKey := fmt.Sprintf(common.CacheKubexmK8sLeafCertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
		if val, ok := ctx.GetModuleCache().Get(leafCacheKey); ok {
			if renew, isBool := val.(bool); isBool && renew {
				renewalTriggered = true
			}
		}
	}

	if !renewalTriggered {
		ctx.GetLogger().Info("Skipping cleanup task: No certificate renewal was performed.")
	}

	return renewalTriggered, nil
}

func (t *CleanupTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

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
		Hosts: []connector.Host{controlNode},
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
