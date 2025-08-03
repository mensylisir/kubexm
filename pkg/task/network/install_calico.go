package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/network/calico"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallCalicoTask struct {
	task.Base
}

func NewInstallCalicoTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallCalicoTask{
		Base: task.Base{
			Name:   "InstallCalico",
			Desc:   "Install Calico CNI plugin",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(InstallCalicoAction),
		},
	}
	return s, nil
}

type InstallCalicoAction struct {
	task.Action
}

func (a *InstallCalicoAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Calico Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	downloadNode := plan.NodeID("download-calico")
	p.AddNode(downloadNode, &plan.ExecutionNode{Step: calico.NewDownloadCalicoStep(ctx, downloadNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	generateNode := plan.NodeID("generate-calico-manifests")
	p.AddNode(generateNode, &plan.ExecutionNode{Step: calico.NewGenerateCalicoManifestStep(ctx, generateNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{downloadNode}})

	installNode := plan.NodeID("install-calico")
	p.AddNode(installNode, &plan.ExecutionNode{Step: calico.NewInstallCalicoStep(ctx, installNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{generateNode}})

	return p, nil
}
