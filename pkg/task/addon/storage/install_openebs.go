package storage

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/storage/openebs-local"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallOpenEBSTask struct {
	task.Base
}

func NewInstallOpenEBSTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallOpenEBSTask{
		Base: task.Base{
			Name:   "InstallOpenEBS",
			Desc:   "Install OpenEBS storage addon",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallOpenEBSAction),
		},
	}
	return s, nil
}

type InstallOpenEBSAction struct {
	task.Action
}

func (a *InstallOpenEBSAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install OpenEBS Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	downloadStep := openebslocal.NewDownloadOpenEBSStep(ctx, "DownloadOpenEBS")
	p.AddNode("download-openebs", &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{controlPlaneHost}})

	generateStep := openebslocal.NewGenerateManifestsStep(ctx, "GenerateOpenEBSManifests")
	p.AddNode("generate-openebs-manifests", &plan.ExecutionNode{Step: generateStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-openebs"}})

	installStep := openebslocal.NewInstallOpenEBSStep(ctx, "InstallOpenEBS")
	p.AddNode("install-openebs", &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"generate-openebs-manifests"}})

	return p, nil
}
