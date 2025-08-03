package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/network/flannel"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallFlannelTask struct {
	task.Base
}

func NewInstallFlannelTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallFlannelTask{
		Base: task.Base{
			Name:   "InstallFlannel",
			Desc:   "Install Flannel CNI plugin",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(InstallFlannelAction),
		},
	}
	return s, nil
}

type InstallFlannelAction struct {
	task.Action
}

func (a *InstallFlannelAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Flannel Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	downloadNode := plan.NodeID("download-flannel")
	p.AddNode(downloadNode, &plan.ExecutionNode{Step: flannel.NewDownloadFlannelStep(ctx, downloadNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	generateNode := plan.NodeID("generate-flannel-manifests")
	p.AddNode(generateNode, &plan.ExecutionNode{Step: flannel.NewGenerateManifestsStep(ctx, generateNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{downloadNode}})

	installNode := plan.NodeID("install-flannel")
	p.AddNode(installNode, &plan.ExecutionNode{Step: flannel.NewInstallFlannelStep(ctx, installNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{generateNode}})

	return p, nil
}
