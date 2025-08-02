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

	downloadStep := flannel.NewDownloadFlannelStep(ctx, "DownloadFlannel")
	p.AddNode("download-flannel", &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{controlPlaneHost}})

	generateStep := flannel.NewGenerateManifestsStep(ctx, "GenerateFlannelManifests")
	p.AddNode("generate-flannel-manifests", &plan.ExecutionNode{Step: generateStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-flannel"}})

	installStep := flannel.NewInstallFlannelStep(ctx, "InstallFlannel")
	p.AddNode("install-flannel", &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"generate-flannel-manifests"}})

	return p, nil
}
