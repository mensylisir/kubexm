package cilium

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/network/cilium"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallCiliumTask installs the Cilium CNI plugin.
type InstallCiliumTask struct {
	task.BaseTask
}

// NewInstallCiliumTask creates a new InstallCiliumTask.
func NewInstallCiliumTask() task.Task {
	return &InstallCiliumTask{
		BaseTask: task.NewBaseTask(
			"InstallCilium",
			"Install the Cilium CNI plugin.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallCiliumTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Cilium Helm chart
	downloadStep := cilium.NewDownloadCiliumChartStepBuilder(ctx, "DownloadCiliumChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Cilium values.yaml
	generateStep := cilium.NewGenerateCiliumValuesStepBuilder(ctx, "GenerateCiliumValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Cilium artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := cilium.NewDistributeCiliumArtifactsStepBuilder(ctx, "DistributeCiliumArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Cilium Helm chart
	installStep := cilium.NewInstallCiliumHelmChartStepBuilder(ctx, "InstallCiliumChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Cilium installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallCiliumTask)(nil)
