package longhorn

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/storage/longhorn"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallLonghornTask installs the Longhorn addon.
type InstallLonghornTask struct {
	task.BaseTask
}

// NewInstallLonghornTask creates a new InstallLonghornTask.
func NewInstallLonghornTask() task.Task {
	return &InstallLonghornTask{
		BaseTask: task.NewBaseTask(
			"InstallLonghorn",
			"Install the Longhorn addon.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallLonghornTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Longhorn Helm chart
	downloadStep := longhorn.NewDownloadLonghornChartStepBuilder(ctx, "DownloadLonghornChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Longhorn values.yaml
	generateStep := longhorn.NewGenerateLonghornValuesStepBuilder(ctx, "GenerateLonghornValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Longhorn artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := longhorn.NewDistributeLonghornArtifactsStepBuilder(ctx, "DistributeLonghornArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Longhorn Helm chart
	installStep := longhorn.NewInstallLonghornHelmChartStepBuilder(ctx, "InstallLonghornChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Longhorn installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallLonghornTask)(nil)
