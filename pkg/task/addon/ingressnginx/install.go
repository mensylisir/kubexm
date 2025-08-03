package ingressnginx

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/gateway/ingress-nginx"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallIngressNginxTask struct {
	task.Base
}

func NewInstallIngressNginxTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallIngressNginxTask{
		Base: task.Base{
			Name:   "InstallIngressNginx",
			Desc:   "Install the Ingress-Nginx addon using manifests",
			Hosts:  ctx.GetHosts(),
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

	downloadNode := plan.NodeID("download-ingress-nginx")
	p.AddNode(downloadNode, &plan.ExecutionNode{
		Step:  ingressnginx.NewDownloadIngressNginxStep(ctx, downloadNode.String()),
		Hosts: []connector.Host{controlPlaneHost},
	})

	generateNode := plan.NodeID("generate-ingress-nginx-manifests")
	p.AddNode(generateNode, &plan.ExecutionNode{
		Step:         ingressnginx.NewGenerateManifestsStep(ctx, generateNode.String()),
		Hosts:        []connector.Host{controlPlaneHost},
		Dependencies: []plan.NodeID{downloadNode},
	})

	installNode := plan.NodeID("install-ingress-nginx")
	p.AddNode(installNode, &plan.ExecutionNode{
		Step:         ingressnginx.NewInstallIngressNginxStep(ctx, installNode.String()),
		Hosts:        []connector.Host{controlPlaneHost},
		Dependencies: []plan.NodeID{generateNode},
	})

	return p, nil
}
