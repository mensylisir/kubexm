package kubeovn

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/network/kubeovn"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallKubeOVNTask installs the Kube-OVN CNI plugin.
type InstallKubeOVNTask struct {
	task.BaseTask
}

// NewInstallKubeOVNTask creates a new InstallKubeOVNTask.
func NewInstallKubeOVNTask() task.Task {
	return &InstallKubeOVNTask{
		BaseTask: task.NewBaseTask(
			"InstallKubeOVN",
			"Install the Kube-OVN CNI plugin.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallKubeOVNTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Kube-OVN Helm chart
	downloadStep := kubeovn.NewDownloadKubeovnChartStepBuilder(ctx, "DownloadKubeOVNChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Kube-OVN values.yaml
	generateStep := kubeovn.NewGenerateKubeovnValuesStepBuilder(ctx, "GenerateKubeOVNValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Kube-OVN artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := kubeovn.NewDistributeKubeovnArtifactsStepBuilder(ctx, "DistributeKubeOVNArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Kube-OVN Helm chart
	installStep := kubeovn.NewInstallKubeOvnHelmChartStepBuilder(ctx, "InstallKubeOVNChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Kube-OVN installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallKubeOVNTask)(nil)
