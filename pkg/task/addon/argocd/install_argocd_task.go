package argocd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/cd/argocd"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallArgoCDTask installs the Argo CD addon.
type InstallArgoCDTask struct {
	task.BaseTask
}

// NewInstallArgoCDTask creates a new InstallArgoCDTask.
func NewInstallArgoCDTask() task.Task {
	return &InstallArgoCDTask{
		BaseTask: task.NewBaseTask(
			"InstallArgoCD",
			"Install the Argo CD addon.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallArgoCDTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Argo CD Helm chart
	downloadStep := argocd.NewDownloadArgoCDChartStepBuilder(ctx, "DownloadArgoCDChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Argo CD values.yaml
	generateStep := argocd.NewGenerateArgoCDValuesStepBuilder(ctx, "GenerateArgoCDValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Argo CD artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := argocd.NewDistributeArgoCDArtifactsStepBuilder(ctx, "DistributeArgoCDArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Argo CD Helm chart
	installStep := argocd.NewInstallArgoCDHelmChartStepBuilder(ctx, "InstallArgoCDChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Argo CD installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallArgoCDTask)(nil)
