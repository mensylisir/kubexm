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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy nfs-provisioner")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues := nfsstep.NewGenerateNFSProvisionerValuesStepBuilder(*runtimeCtx, "GenerateNFSProvisionerValues").Build()
	distributeArtifacts := nfsstep.NewDistributeNFSProvisionerArtifactsStepBuilder(*runtimeCtx, "DistributeNFSProvisionerArtifacts").Build()
	installChart := nfsstep.NewInstallNFSProvisionerHelmChartStepBuilder(*runtimeCtx, "InstallNFSProvisionerChart").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNFSProvisionerValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeNFSProvisionerArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNFSProvisionerChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateNFSProvisionerValues", "DistributeNFSProvisionerArtifacts")
	fragment.AddDependency("DistributeNFSProvisionerArtifacts", "InstallNFSProvisionerChart")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading NFS Provisioner chart.")
		downloadChart := nfsstep.NewDownloadNFSProvisionerChartStepBuilder(*runtimeCtx, "DownloadNFSProvisionerChart").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadNFSProvisionerChart", Step: downloadChart, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadNFSProvisionerChart", "GenerateNFSProvisionerValues")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download for NFS Provisioner artifacts.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
