package nfs

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	nfsstep "github.com/mensylisir/kubexm/pkg/step/storage/nfs"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanNfsTask struct {
	task.Base
}

func NewCleanNfsTask() task.Task {
	return &CleanNfsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanNfsProvisioner",
				Description: "Clean up nfs-subdir-external-provisioner using Helm",
			},
		},
	}
}

func (t *CleanNfsTask) Name() string {
	return t.Meta.Name
}

func (t *CleanNfsTask) Description() string {
	return t.Meta.Description
}

func (t *CleanNfsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.NFS == nil || cfg.Spec.Storage.NFS.Enabled == nil {
		return false, nil
	}
	return *cfg.Spec.Storage.NFS.Enabled, nil
}

func (t *CleanNfsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep := nfsstep.NewCleanNFSProvisionerStepBuilder(*runtimeCtx, "UninstallNFSProvisionerRelease").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallNFSProvisionerRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
