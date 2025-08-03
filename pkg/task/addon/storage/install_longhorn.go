package storage

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/storage/longhorn"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallLonghornTask struct {
	task.Base
}

func NewInstallLonghornTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallLonghornTask{
		Base: task.Base{
			Name:   "InstallLonghorn",
			Desc:   "Install Longhorn storage addon",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallLonghornAction),
		},
	}
	return s, nil
}

type InstallLonghornAction struct {
	task.Action
}

func (a *InstallLonghornAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Longhorn Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	downloadNode := plan.NodeID("download-longhorn")
	p.AddNode(downloadNode, &plan.ExecutionNode{Step: longhorn.NewDownloadLonghornStep(ctx, downloadNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	generateNode := plan.NodeID("generate-longhorn-manifests")
	p.AddNode(generateNode, &plan.ExecutionNode{Step: longhorn.NewGenerateManifestsStep(ctx, generateNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{downloadNode}})

	installNode := plan.NodeID("install-longhorn")
	p.AddNode(installNode, &plan.ExecutionNode{Step: longhorn.NewInstallLonghornStep(ctx, installNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{generateNode}})

	return p, nil
}
