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

	downloadStep := nfs.NewDownloadNFSStep(ctx, "DownloadNFS")
	p.AddNode("download-nfs", &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{controlPlaneHost}})

	generateStep := nfs.NewGenerateManifestsStep(ctx, "GenerateNFSManifests")
	p.AddNode("generate-nfs-manifests", &plan.ExecutionNode{Step: generateStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-nfs"}})

	installStep := nfs.NewInstallNFSStep(ctx, "InstallNFS")
	p.AddNode("install-nfs", &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"generate-nfs-manifests"}})

	return p, nil
}
