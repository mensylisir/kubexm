package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/pkg/step/loadbalancer/keepalived"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetupExternalLoadBalancerTask struct {
	task.Base
}

func NewSetupExternalLoadBalancerTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &SetupExternalLoadBalancerTask{
		Base: task.Base{
			Name:   "SetupExternalLoadBalancer",
			Desc:   "Setup Keepalived and HAProxy/Nginx on dedicated load balancer nodes",
			Hosts:  ctx.GetHostsByRole(common.RoleLoadBalancer),
			Action: new(SetupExternalLoadBalancerAction),
		},
	}
	return s, nil
}

type SetupExternalLoadBalancerAction struct {
	task.Action
}

func (a *SetupExternalLoadBalancerAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Setup External Load Balancer Phase")

	lbHosts := a.GetHosts()
	if len(lbHosts) == 0 {
		return p, nil
	}

	// This task assumes HAProxy. A real implementation would switch.
	for _, host := range lbHosts {
		hostName := host.GetName()

		// --- Keepalived ---
		installKeepalivedNode := plan.NodeID(fmt.Sprintf("install-keepalived-%s", hostName))
		p.AddNode(installKeepalivedNode, &plan.ExecutionNode{Step: keepalived.NewInstallKeepalivedStep(ctx, installKeepalivedNode.String()), Hosts: []connector.Host{host}})

		genKeepalivedCfgNode := plan.NodeID(fmt.Sprintf("gen-keepalived-cfg-%s", hostName))
		p.AddNode(genKeepalivedCfgNode, &plan.ExecutionNode{Step: keepalived.NewGenerateKeepalivedConfigStep(ctx, genKeepalivedCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installKeepalivedNode}})

		deployKeepalivedCfgNode := plan.NodeID(fmt.Sprintf("deploy-keepalived-cfg-%s", hostName))
		p.AddNode(deployKeepalivedCfgNode, &plan.ExecutionNode{Step: keepalived.NewDeployKeepalivedConfigStep(ctx, deployKeepalivedCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genKeepalivedCfgNode}})

		enableKeepalivedNode := plan.NodeID(fmt.Sprintf("enable-keepalived-%s", hostName))
		p.AddNode(enableKeepalivedNode, &plan.ExecutionNode{Step: keepalived.NewEnableKeepalivedStep(ctx, enableKeepalivedNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{deployKeepalivedCfgNode}})

		restartKeepalivedNode := plan.NodeID(fmt.Sprintf("restart-keepalived-%s", hostName))
		p.AddNode(restartKeepalivedNode, &plan.ExecutionNode{Step: keepalived.NewRestartKeepalivedStep(ctx, restartKeepalivedNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableKeepalivedNode}})

		// --- HAProxy ---
		installHAProxyNode := plan.NodeID(fmt.Sprintf("install-haproxy-%s", hostName))
		p.AddNode(installHAProxyNode, &plan.ExecutionNode{Step: haproxy.NewInstallHAProxyStep(ctx, installHAProxyNode.String()), Hosts: []connector.Host{host}})

		genHAProxyCfgNode := plan.NodeID(fmt.Sprintf("gen-haproxy-cfg-%s", hostName))
		p.AddNode(genHAProxyCfgNode, &plan.ExecutionNode{Step: haproxy.NewGenerateHAProxyConfigStep(ctx, genHAProxyCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installHAProxyNode}})

		deployHAProxyCfgNode := plan.NodeID(fmt.Sprintf("deploy-haproxy-cfg-%s", hostName))
		p.AddNode(deployHAProxyCfgNode, &plan.ExecutionNode{Step: haproxy.NewDeployHAProxyConfigStep(ctx, deployHAProxyCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genHAProxyCfgNode}})

		enableHAProxyNode := plan.NodeID(fmt.Sprintf("enable-haproxy-%s", hostName))
		p.AddNode(enableHAProxyNode, &plan.ExecutionNode{Step: haproxy.NewEnableHAProxyStep(ctx, enableHAProxyNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{deployHAProxyCfgNode}})

		restartHAProxyNode := plan.NodeID(fmt.Sprintf("restart-haproxy-%s", hostName))
		p.AddNode(restartHAProxyNode, &plan.ExecutionNode{Step: haproxy.NewRestartHAProxyStep(ctx, restartHAProxyNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{restartKeepalivedNode, enableHAProxyNode}})
	}

	return p, nil
}
