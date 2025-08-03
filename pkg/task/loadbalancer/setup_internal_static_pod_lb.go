package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetupInternalStaticPodLBTask struct {
	task.Base
}

func NewSetupInternalStaticPodLBTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &SetupInternalStaticPodLBTask{
		Base: task.Base{
			Name:   "SetupInternalStaticPodLoadBalancer",
			Desc:   "Setup HAProxy/Nginx as static pods on master nodes",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(SetupInternalStaticPodLBAction),
		},
	}
	return s, nil
}

type SetupInternalStaticPodLBAction struct {
	task.Action
}

func (a *SetupInternalStaticPodLBAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Setup Internal Static Pod LB Phase")

	masterHosts := a.GetHosts()
	if len(masterHosts) == 0 {
		return p, nil
	}

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	genCfgNode := plan.NodeID("gen-haproxy-cfg-for-static-pod")
	p.AddNode(genCfgNode, &plan.ExecutionNode{Step: haproxy.NewGenerateHAProxyConfigStep(ctx, genCfgNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	deployCfgNode := plan.NodeID("deploy-haproxy-cfg-for-static-pod")
	p.AddNode(deployCfgNode, &plan.ExecutionNode{Step: haproxy.NewDeployHAProxyConfigStep(ctx, deployCfgNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{genCfgNode}})

	genManifestNode := plan.NodeID("gen-haproxy-manifest")
	p.AddNode(genManifestNode, &plan.ExecutionNode{Step: haproxy.NewGenerateHAProxyStaticPodManifestStep(ctx, genManifestNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	deployManifestNode := plan.NodeID("deploy-haproxy-manifest")
	p.AddNode(deployManifestNode, &plan.ExecutionNode{Step: haproxy.NewDeployHAProxyStaticPodManifestStep(ctx, deployManifestNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{genManifestNode, deployCfgNode}})

	return p, nil
}
