package nfs

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	nfsstep "github.com/mensylisir/kubexm/internal/step/storage/nfs"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep, err := nfsstep.NewCleanNFSProvisionerStepBuilder(runtimeCtx, "UninstallNFSProvisionerRelease").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallNFSProvisionerRelease", Step: cleanStep, Hosts: []remotefw.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
