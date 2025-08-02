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

	downloadStep := cilium.NewDownloadCiliumStep(ctx, "DownloadCilium")
	p.AddNode("download-cilium", &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{controlPlaneHost}})

	generateStep := cilium.NewGenerateManifestsStep(ctx, "GenerateCiliumManifests")
	p.AddNode("generate-cilium-manifests", &plan.ExecutionNode{Step: generateStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-cilium"}})

	installStep := cilium.NewInstallCiliumStep(ctx, "InstallCilium")
	p.AddNode("install-cilium", &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"generate-cilium-manifests"}})

	return p, nil
}
