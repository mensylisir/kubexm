package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/network/cilium"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallCiliumTask struct {
	task.Base
}

func NewInstallCiliumTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallCiliumTask{
		Base: task.Base{
			Name:   "InstallCilium",
			Desc:   "Install Cilium CNI plugin",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(InstallCiliumAction),
		},
	}
	return s, nil
}

type InstallCiliumAction struct {
	task.Action
}

func (a *InstallCiliumAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Cilium Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	downloadNode := plan.NodeID("download-cilium")
	p.AddNode(downloadNode, &plan.ExecutionNode{Step: cilium.NewDownloadCiliumStep(ctx, downloadNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	generateNode := plan.NodeID("generate-cilium-manifests")
	p.AddNode(generateNode, &plan.ExecutionNode{Step: cilium.NewGenerateManifestsStep(ctx, generateNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{downloadNode}})

	installNode := plan.NodeID("install-cilium")
	p.AddNode(installNode, &plan.ExecutionNode{Step: cilium.NewInstallCiliumStep(ctx, installNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{generateNode}})

	return p, nil
}
