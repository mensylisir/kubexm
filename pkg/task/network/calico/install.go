package calico

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/network/calico"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallCalicoTask installs the Calico CNI plugin.
type InstallCalicoTask struct {
	task.BaseTask
}

// NewInstallCalicoTask creates a new InstallCalicoTask.
func NewInstallCalicoTask() task.Task {
	return &InstallCalicoTask{
		BaseTask: task.NewBaseTask(
			"InstallCalico",
			"Install the Calico CNI plugin.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallCalicoTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Calico Helm chart (local step on control plane)
	downloadStep := calico.NewDownloadCalicoChartStepBuilder(ctx, "DownloadCalicoChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Calico values.yaml (local step on control plane)
	generateStep := calico.NewGenerateCalicoValuesStepBuilder(ctx, "GenerateCalicoValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Calico artifacts (to the first master node)
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := calico.NewDistributeCalicoArtifactsStepBuilder(ctx, "DistributeCalicoArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Calico Helm chart (on the first master node)
	installStep := calico.NewInstallCalicoHelmChartStepBuilder(ctx, "InstallCalicoChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Calico installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallCalicoTask)(nil)
