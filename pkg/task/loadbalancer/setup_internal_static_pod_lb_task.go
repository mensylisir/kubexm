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

	// This task assumes HAProxy. A real implementation would switch between haproxy and nginx.

	// First, generate the haproxy.cfg and deploy it to all master nodes.
	// The generation step can be run on the control node.
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	genCfg := haproxy.NewGenerateHAProxyConfigStep(ctx, "GenerateHAProxyConfigForStaticPod")
	p.AddNode("gen-haproxy-cfg", &plan.ExecutionNode{Step: genCfg, Hosts: []connector.Host{controlNode}})

	deployCfg := haproxy.NewDeployHAProxyConfigStep(ctx, "DeployHAProxyConfigForStaticPod")
	p.AddNode("deploy-haproxy-cfg", &plan.ExecutionNode{Step: deployCfg, Hosts: masterHosts, Dependencies: []plan.NodeID{"gen-haproxy-cfg"}})

	// Then, generate and deploy the static pod manifest to all master nodes.
	genManifest := haproxy.NewGenerateHAProxyStaticPodManifestStep(ctx, "GenerateHAProxyStaticPodManifest")
	p.AddNode("gen-haproxy-manifest", &plan.ExecutionNode{Step: genManifest, Hosts: []connector.Host{controlNode}})

	deployManifest := haproxy.NewDeployHAProxyStaticPodManifestStep(ctx, "DeployHAProxyStaticPodManifest")
	p.AddNode("deploy-haproxy-manifest", &plan.ExecutionNode{Step: deployManifest, Hosts: masterHosts, Dependencies: []plan.NodeID{"gen-haproxy-manifest", "deploy-haproxy-cfg"}})

	return p, nil
}
