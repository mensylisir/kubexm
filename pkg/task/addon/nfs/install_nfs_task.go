package nfs

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/storage/nfs"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallNFSTask installs the NFS Provisioner addon.
type InstallNFSTask struct {
	task.BaseTask
}

// NewInstallNFSTask creates a new InstallNFSTask.
func NewInstallNFSTask() task.Task {
	return &InstallNFSTask{
		BaseTask: task.NewBaseTask(
			"InstallNFSProvisioner",
			"Install the NFS Provisioner addon.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallNFSTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download NFS Provisioner Helm chart
	downloadStep := nfs.NewDownloadNFSProvisionerChartStepBuilder(ctx, "DownloadNFSProvisionerChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate NFS Provisioner values.yaml
	generateStep := nfs.NewGenerateNFSProvisionerValuesStepBuilder(ctx, "GenerateNFSProvisionerValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute NFS Provisioner artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := nfs.NewDistributeNFSProvisionerArtifactsStepBuilder(ctx, "DistributeNFSProvisionerArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install NFS Provisioner Helm chart
	installStep := nfs.NewInstallNFSProvisionerHelmChartStepBuilder(ctx, "InstallNFSProvisionerChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("NFS Provisioner installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallNFSTask)(nil)
