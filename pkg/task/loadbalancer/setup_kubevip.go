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

	genManifestNode := plan.NodeID("gen-kubevip-manifest")
	p.AddNode(genManifestNode, &plan.ExecutionNode{Step: kubevip.NewGenerateKubeVipManifestStep(ctx, genManifestNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	deployManifestNode := plan.NodeID("deploy-kubevip-manifest")
	p.AddNode(deployManifestNode, &plan.ExecutionNode{Step: kubevip.NewDeployKubeVipManifestStep(ctx, deployManifestNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{genManifestNode}})

	return p, nil
}
