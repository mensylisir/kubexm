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
		// No external LB nodes, so this task does nothing.
		return p, nil
	}

	// For now, this task assumes HAProxy. A real implementation would have a switch
	// based on the cluster spec to choose between haproxy and nginx steps.

	for _, host := range lbHosts {
		hostName := host.GetName()

		// --- Keepalived Installation Chain ---
		installKeepalived := keepalived.NewInstallKeepalivedStep(ctx, fmt.Sprintf("InstallKeepalived-%s", hostName))
		p.AddNode(plan.NodeID(installKeepalived.Meta().Name), &plan.ExecutionNode{Step: installKeepalived, Hosts: []connector.Host{host}})

		genKeepalivedCfg := keepalived.NewGenerateKeepalivedConfigStep(ctx, fmt.Sprintf("GenerateKeepalivedConfig-%s", hostName))
		p.AddNode(plan.NodeID(genKeepalivedCfg.Meta().Name), &plan.ExecutionNode{Step: genKeepalivedCfg, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installKeepalived.Meta().Name)}})

		deployKeepalivedCfg := keepalived.NewDeployKeepalivedConfigStep(ctx, fmt.Sprintf("DeployKeepalivedConfig-%s", hostName))
		p.AddNode(plan.NodeID(deployKeepalivedCfg.Meta().Name), &plan.ExecutionNode{Step: deployKeepalivedCfg, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(genKeepalivedCfg.Meta().Name)}})

		enableKeepalived := keepalived.NewEnableKeepalivedStep(ctx, fmt.Sprintf("EnableKeepalived-%s", hostName))
		p.AddNode(plan.NodeID(enableKeepalived.Meta().Name), &plan.ExecutionNode{Step: enableKeepalived, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(deployKeepalivedCfg.Meta().Name)}})

		restartKeepalived := keepalived.NewRestartKeepalivedStep(ctx, fmt.Sprintf("RestartKeepalived-%s", hostName))
		p.AddNode(plan.NodeID(restartKeepalived.Meta().Name), &plan.ExecutionNode{Step: restartKeepalived, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(enableKeepalived.Meta().Name)}})
		keepalivedReadyNode := plan.NodeID(restartKeepalived.Meta().Name)

		// --- HAProxy Installation Chain ---
		installHAProxy := haproxy.NewInstallHAProxyStep(ctx, fmt.Sprintf("InstallHAProxy-%s", hostName))
		p.AddNode(plan.NodeID(installHAProxy.Meta().Name), &plan.ExecutionNode{Step: installHAProxy, Hosts: []connector.Host{host}})

		genHAProxyCfg := haproxy.NewGenerateHAProxyConfigStep(ctx, fmt.Sprintf("GenerateHAProxyConfig-%s", hostName))
		p.AddNode(plan.NodeID(genHAProxyCfg.Meta().Name), &plan.ExecutionNode{Step: genHAProxyCfg, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installHAProxy.Meta().Name)}})

		deployHAProxyCfg := haproxy.NewDeployHAProxyConfigStep(ctx, fmt.Sprintf("DeployHAProxyConfig-%s", hostName))
		p.AddNode(plan.NodeID(deployHAProxyCfg.Meta().Name), &plan.ExecutionNode{Step: deployHAProxyCfg, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(genHAProxyCfg.Meta().Name)}})

		enableHAProxy := haproxy.NewEnableHAProxyStep(ctx, fmt.Sprintf("EnableHAProxy-%s", hostName))
		p.AddNode(plan.NodeID(enableHAProxy.Meta().Name), &plan.ExecutionNode{Step: enableHAProxy, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(deployHAProxyCfg.Meta().Name)}})

		// HAProxy should restart after Keepalived is up, to ensure VIP is available.
		restartHAProxy := haproxy.NewRestartHAProxyStep(ctx, fmt.Sprintf("RestartHAProxy-%s", hostName))
		p.AddNode(plan.NodeID(restartHAProxy.Meta().Name), &plan.ExecutionNode{Step: restartHAProxy, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{keepalivedReadyNode, plan.NodeID(enableHAProxy.Meta().Name)}})
	}

	return p, nil
}
