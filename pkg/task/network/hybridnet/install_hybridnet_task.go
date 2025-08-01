package hybridnet

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/network/hybridnet"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallHybridnetTask installs the Hybridnet CNI plugin.
type InstallHybridnetTask struct {
	task.BaseTask
}

// NewInstallHybridnetTask creates a new InstallHybridnetTask.
func NewInstallHybridnetTask() task.Task {
	return &InstallHybridnetTask{
		BaseTask: task.NewBaseTask(
			"InstallHybridnet",
			"Install the Hybridnet CNI plugin.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallHybridnetTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Hybridnet Helm chart
	downloadStep := hybridnet.NewDownloadHybridnetChartStepBuilder(ctx, "DownloadHybridnetChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Hybridnet values.yaml
	generateStep := hybridnet.NewGenerateHybridnetValuesStepBuilder(ctx, "GenerateHybridnetValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Hybridnet artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := hybridnet.NewDistributeHybridnetArtifactsStepBuilder(ctx, "DistributeHybridnetArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Hybridnet Helm chart
	installStep := hybridnet.NewInstallHybridnetHelmChartStepBuilder(ctx, "InstallHybridnetChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Hybridnet installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallHybridnetTask)(nil)
