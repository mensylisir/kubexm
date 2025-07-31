package openebs

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/storage/openebs-local"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallOpenEBSTask installs the OpenEBS addon.
type InstallOpenEBSTask struct {
	task.BaseTask
}

// NewInstallOpenEBSTask creates a new InstallOpenEBSTask.
func NewInstallOpenEBSTask() task.Task {
	return &InstallOpenEBSTask{
		BaseTask: task.NewBaseTask(
			"InstallOpenEBS",
			"Install the OpenEBS addon.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallOpenEBSTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download OpenEBS Helm chart
	downloadStep := openebslocal.NewDownloadOpenEBSChartStepBuilder(ctx, "DownloadOpenEBSChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate OpenEBS values.yaml
	generateStep := openebslocal.NewGenerateOpenEBSValuesStepBuilder(ctx, "GenerateOpenEBSValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute OpenEBS artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := openebslocal.NewDistributeOpenEBSArtifactsStepBuilder(ctx, "DistributeOpenEBSArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install OpenEBS Helm chart
	installStep := openebslocal.NewInstallOpenEBSHelmChartStepBuilder(ctx, "InstallOpenEBSChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("OpenEBS installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallOpenEBSTask)(nil)
