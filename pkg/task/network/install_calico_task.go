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
			Hosts:  ctx.GetHostsByRole(common.RoleMaster), // Run on one master
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

	// The installation steps should only be run from one master node.
	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// 1. Download Calico resources
	downloadCalico := calico.NewDownloadCalicoStep(ctx, "DownloadCalico")
	p.AddNode("download-calico", &plan.ExecutionNode{Step: downloadCalico, Hosts: []connector.Host{controlPlaneHost}})

	// 2. Generate the manifest
	// This step might take the cluster config to customize the Calico manifest (e.g., Pod CIDR)
	genManifest := calico.NewGenerateCalicoManifestStep(ctx, "GenerateCalicoManifest")
	p.AddNode("gen-calico-manifest", &plan.ExecutionNode{Step: genManifest, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-calico"}})

	// 3. Apply the manifest to the cluster
	installCalico := calico.NewInstallCalicoStep(ctx, "InstallCalico")
	p.AddNode("install-calico", &plan.ExecutionNode{Step: installCalico, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"gen-calico-manifest"}})

	return p, nil
}
