package ingressnginx

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/gateway/ingress-nginx"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallIngressNginxTask installs the Ingress-Nginx addon.
type InstallIngressNginxTask struct {
	task.Base
}

func NewInstallIngressNginxTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallIngressNginxTask{
		Base: task.Base{
			Name:   "InstallIngressNginx",
			Desc:   "Install the Ingress-Nginx addon using manifests",
			Hosts:  ctx.GetHosts(), // This task will only run on the control node, but gets all hosts for context
			Action: new(InstallIngressNginxAction),
		},
	}
	return s, nil
}

type InstallIngressNginxAction struct {
	task.Action
}

func (a *InstallIngressNginxAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Ingress-Nginx Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", a.Name, err)
	}

	// 1. Download Ingress-Nginx resources (e.g., the deploy.yaml manifest)
	downloadStep := ingressnginx.NewDownloadIngressNginxStep(ctx, "DownloadIngressNginxResources")
	p.AddNode("download-ingress-nginx", &plan.ExecutionNode{
		Step:  downloadStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// 2. Generate/Customize manifests if needed.
	// This step could modify the downloaded manifest, e.g., to change node ports or replicas.
	generateStep := ingressnginx.NewGenerateManifestsStep(ctx, "GenerateIngressNginxManifests")
	p.AddNode("generate-ingress-nginx-manifests", &plan.ExecutionNode{
		Step:         generateStep,
		Hosts:        []connector.Host{controlPlaneHost},
		Dependencies: []plan.NodeID{"download-ingress-nginx"},
	})

	// 3. Install Ingress-Nginx by applying the manifest.
	installStep := ingressnginx.NewInstallIngressNginxStep(ctx, "InstallIngressNginx")
	p.AddNode("install-ingress-nginx", &plan.ExecutionNode{
		Step:         installStep,
		Hosts:        []connector.Host{controlPlaneHost},
		Dependencies: []plan.NodeID{"generate-ingress-nginx-manifests"},
	})

	return p, nil
}
