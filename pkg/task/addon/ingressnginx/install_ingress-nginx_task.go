package ingressnginx

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/gateway/ingress-nginx"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallIngressNginxTask installs the Ingress-Nginx addon.
type InstallIngressNginxTask struct {
	task.BaseTask
}

// NewInstallIngressNginxTask creates a new InstallIngressNginxTask.
func NewInstallIngressNginxTask() task.Task {
	return &InstallIngressNginxTask{
		BaseTask: task.NewBaseTask(
			"InstallIngressNginx",
			"Install the Ingress-Nginx addon.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallIngressNginxTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// Step 1: Download Ingress-Nginx Helm chart
	downloadStep := ingressnginx.NewDownloadIngressNginxChartStepBuilder(ctx, "DownloadIngressNginxChart").Build()
	downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  downloadStep.Meta().Name,
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Ingress-Nginx values.yaml
	generateStep := ingressnginx.NewGenerateIngressNginxValuesStepBuilder(ctx, "GenerateIngressNginxValues").Build()
	generateNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  generateStep.Meta().Name,
		Step:  generateStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 3: Distribute Ingress-Nginx artifacts
	master, err := ctx.GetFirstMaster()
	if err != nil {
		return nil, fmt.Errorf("failed to get a master node for task %s: %w", t.Name(), err)
	}
	distributeStep := ingressnginx.NewDistributeIngressNginxArtifactsStepBuilder(ctx, "DistributeIngressNginxArtifacts").Build()
	distributeNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         distributeStep.Meta().Name,
		Step:         distributeStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{downloadNodeID, generateNodeID},
	})

	// Step 4: Install Ingress-Nginx Helm chart
	installStep := ingressnginx.NewInstallIngressNginxHelmChartStepBuilder(ctx, "InstallIngressNginxChart").Build()
	installNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         installStep.Meta().Name,
		Step:         installStep,
		Hosts:        []connector.Host{master},
		Dependencies: []plan.NodeID{distributeNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{downloadNodeID, generateNodeID}
	fragment.ExitNodes = []plan.NodeID{installNodeID}

	logger.Info("Ingress-Nginx installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallIngressNginxTask)(nil)
