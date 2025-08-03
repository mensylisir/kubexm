package storage

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/storage/nfs"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallNFSTask struct {
	task.Base
}

func NewInstallNFSTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallNFSTask{
		Base: task.Base{
			Name:   "InstallNFS",
			Desc:   "Install NFS storage addon",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallNFSAction),
		},
	}
	return s, nil
}

type InstallNFSAction struct {
	task.Action
}

func (a *InstallNFSAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install NFS Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	downloadNode := plan.NodeID("download-nfs")
	p.AddNode(downloadNode, &plan.ExecutionNode{Step: nfs.NewDownloadNFSStep(ctx, downloadNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	generateNode := plan.NodeID("generate-nfs-manifests")
	p.AddNode(generateNode, &plan.ExecutionNode{Step: nfs.NewGenerateManifestsStep(ctx, generateNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{downloadNode}})

	installNode := plan.NodeID("install-nfs")
	p.AddNode(installNode, &plan.ExecutionNode{Step: nfs.NewInstallNFSStep(ctx, installNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{generateNode}})

	return p, nil
}
