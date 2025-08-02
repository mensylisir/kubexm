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

	downloadStep := longhorn.NewDownloadLonghornStep(ctx, "DownloadLonghorn")
	p.AddNode("download-longhorn", &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{controlPlaneHost}})

	generateStep := longhorn.NewGenerateManifestsStep(ctx, "GenerateLonghornManifests")
	p.AddNode("generate-longhorn-manifests", &plan.ExecutionNode{Step: generateStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-longhorn"}})

	installStep := longhorn.NewInstallLonghornStep(ctx, "InstallLonghorn")
	p.AddNode("install-longhorn", &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"generate-longhorn-manifests"}})

	return p, nil
}
