package multus

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/network/multus"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallMultusTask installs the Multus CNI plugin.
type InstallMultusTask struct {
	task.BaseTask
}

// NewInstallMultusTask creates a new InstallMultusTask.
func NewInstallMultusTask() task.Task {
	return &InstallMultusTask{
		BaseTask: task.NewBaseTask(
			"InstallMultus",
			"Install the Multus CNI plugin.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallMultusTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Multus Helm chart
	downloadStep := multus.NewDownloadMultusChartStepBuilder(ctx, "DownloadMultusChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Multus values.yaml
	generateStep := multus.NewGenerateMultusValuesStepBuilder(ctx, "GenerateMultusValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Multus artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := multus.NewDistributeMultusArtifactsStepBuilder(ctx, "DistributeMultusArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Multus Helm chart
	installStep := multus.NewInstallMultusHelmChartStepBuilder(ctx, "InstallMultusChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Multus installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallMultusTask)(nil)
