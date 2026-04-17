package nfs

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	nfsstep "github.com/mensylisir/kubexm/internal/step/storage/nfs"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployNfsTask struct {
	task.Base
}

func NewDeployNfsTask() task.Task {
	return &DeployNfsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNfsProvisioner",
				Description: "Deploy nfs-subdir-external-provisioner using Helm",
			},
		},
	}
}

func (t *DeployNfsTask) Name() string {
	return t.Meta.Name
}

func (t *DeployNfsTask) Description() string {
	return t.Meta.Description
}

func (t *DeployNfsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.NFS == nil || cfg.Spec.Storage.NFS.Enabled == nil {
		return false, nil
	}
	return *cfg.Spec.Storage.NFS.Enabled, nil
}

func (t *DeployNfsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy nfs-provisioner")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues, err := nfsstep.NewGenerateNFSProvisionerValuesStepBuilder(runtimeCtx, "GenerateNFSProvisionerValues").Build()
	if err != nil {
		return nil, err
	}
	distributeArtifacts, err := nfsstep.NewDistributeNFSProvisionerArtifactsStepBuilder(runtimeCtx, "DistributeNFSProvisionerArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installChart, err := nfsstep.NewInstallNFSProvisionerHelmChartStepBuilder(runtimeCtx, "InstallNFSProvisionerChart").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNFSProvisionerValues", Step: generateValues, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeNFSProvisionerArtifacts", Step: distributeArtifacts, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNFSProvisionerChart", Step: installChart, Hosts: []remotefw.Host{executionHost}})

	fragment.AddDependency("GenerateNFSProvisionerValues", "DistributeNFSProvisionerArtifacts")
	fragment.AddDependency("DistributeNFSProvisionerArtifacts", "InstallNFSProvisionerChart")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
