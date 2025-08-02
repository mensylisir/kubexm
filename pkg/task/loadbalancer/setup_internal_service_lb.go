package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetupInternalServiceLBTask struct {
	task.Base
}

func NewSetupInternalServiceLBTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &SetupInternalServiceLBTask{
		Base: task.Base{
			Name:   "SetupInternalServiceLoadBalancer",
			Desc:   "Setup HAProxy/Nginx as a system service on master nodes",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(SetupInternalServiceLBAction),
		},
	}
	return s, nil
}

type SetupInternalServiceLBAction struct {
	task.Action
}

func (a *SetupInternalServiceLBAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Setup Internal Service LB Phase")

	masterHosts := a.GetHosts()
	if len(masterHosts) == 0 {
		return p, nil
	}

	// This task assumes HAProxy. A real implementation would switch.
	for _, host := range masterHosts {
		hostName := host.GetName()

		installHAProxy := haproxy.NewInstallHAProxyStep(ctx, fmt.Sprintf("InstallHAProxy-%s", hostName))
		p.AddNode(plan.NodeID(installHAProxy.Meta().Name), &plan.ExecutionNode{Step: installHAProxy, Hosts: []connector.Host{host}})

		genHAProxyCfg := haproxy.NewGenerateHAProxyConfigStep(ctx, fmt.Sprintf("GenerateHAProxyConfig-%s", hostName))
		p.AddNode(plan.NodeID(genHAProxyCfg.Meta().Name), &plan.ExecutionNode{Step: genHAProxyCfg, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installHAProxy.Meta().Name)}})

		deployHAProxyCfg := haproxy.NewDeployHAProxyConfigStep(ctx, fmt.Sprintf("DeployHAProxyConfig-%s", hostName))
		p.AddNode(plan.NodeID(deployHAProxyCfg.Meta().Name), &plan.ExecutionNode{Step: deployHAProxyCfg, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(genHAProxyCfg.Meta().Name)}})

		enableHAProxy := haproxy.NewEnableHAProxyStep(ctx, fmt.Sprintf("EnableHAProxy-%s", hostName))
		p.AddNode(plan.NodeID(enableHAProxy.Meta().Name), &plan.ExecutionNode{Step: enableHAProxy, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(deployHAProxyCfg.Meta().Name)}})

		restartHAProxy := haproxy.NewRestartHAProxyStep(ctx, fmt.Sprintf("RestartHAProxy-%s", hostName))
		p.AddNode(plan.NodeID(restartHAProxy.Meta().Name), &plan.ExecutionNode{Step: restartHAProxy, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(enableHAProxy.Meta().Name)}})
	}

	return p, nil
}
