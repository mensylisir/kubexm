package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/loadbalancer/kubevip"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetupKubeVipTask struct {
	task.Base
}

func NewSetupKubeVipTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &SetupKubeVipTask{
		Base: task.Base{
			Name:   "SetupKubeVip",
			Desc:   "Setup Kube-vip as a static pod on master nodes",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(SetupKubeVipAction),
		},
	}
	return s, nil
}

type SetupKubeVipAction struct {
	task.Action
}

func (a *SetupKubeVipAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Setup Kube-vip Phase")

	masterHosts := a.GetHosts()
	if len(masterHosts) == 0 {
		return p, nil
	}

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// 1. Generate the manifest on the control node.
	genManifest := kubevip.NewGenerateKubeVipManifestStep(ctx, "GenerateKubeVipManifest")
	p.AddNode("gen-kubevip-manifest", &plan.ExecutionNode{Step: genManifest, Hosts: []connector.Host{controlPlaneHost}})

	// 2. Deploy the manifest to all master nodes.
	deployManifest := kubevip.NewDeployKubeVipManifestStep(ctx, "DeployKubeVipManifest")
	p.AddNode("deploy-kubevip-manifest", &plan.ExecutionNode{Step: deployManifest, Hosts: masterHosts, Dependencies: []plan.NodeID{"gen-kubevip-manifest"}})

	return p, nil
}
