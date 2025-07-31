package flannel

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/network/flannel"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallFlannelTask installs the Flannel CNI plugin.
type InstallFlannelTask struct {
	task.BaseTask
}

// NewInstallFlannelTask creates a new InstallFlannelTask.
func NewInstallFlannelTask() task.Task {
	return &InstallFlannelTask{
		BaseTask: task.NewBaseTask(
			"InstallFlannel",
			"Install the Flannel CNI plugin.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallFlannelTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Flannel Helm chart
	downloadStep := flannel.NewDownloadFlannelChartStepBuilder(ctx, "DownloadFlannelChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Flannel values.yaml
	generateStep := flannel.NewGenerateFlannelValuesStepBuilder(ctx, "GenerateFlannelValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Flannel artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := flannel.NewDistributeFlannelArtifactsStepBuilder(ctx, "DistributeFlannelArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Flannel Helm chart
	installStep := flannel.NewInstallFlannelHelmChartStepBuilder(ctx, "InstallFlannelChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Flannel installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallFlannelTask)(nil)
