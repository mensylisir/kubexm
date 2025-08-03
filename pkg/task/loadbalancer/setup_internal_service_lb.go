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

	for _, host := range masterHosts {
		hostName := host.GetName()

		installNode := plan.NodeID(fmt.Sprintf("install-haproxy-%s", hostName))
		p.AddNode(installNode, &plan.ExecutionNode{Step: haproxy.NewInstallHAProxyStep(ctx, installNode.String()), Hosts: []connector.Host{host}})

		genCfgNode := plan.NodeID(fmt.Sprintf("gen-haproxy-cfg-%s", hostName))
		p.AddNode(genCfgNode, &plan.ExecutionNode{Step: haproxy.NewGenerateHAProxyConfigStep(ctx, genCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installNode}})

		deployCfgNode := plan.NodeID(fmt.Sprintf("deploy-haproxy-cfg-%s", hostName))
		p.AddNode(deployCfgNode, &plan.ExecutionNode{Step: haproxy.NewDeployHAProxyConfigStep(ctx, deployCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genCfgNode}})

		enableNode := plan.NodeID(fmt.Sprintf("enable-haproxy-%s", hostName))
		p.AddNode(enableNode, &plan.ExecutionNode{Step: haproxy.NewEnableHAProxyStep(ctx, enableNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{deployCfgNode}})

		restartNode := plan.NodeID(fmt.Sprintf("restart-haproxy-%s", hostName))
		p.AddNode(restartNode, &plan.ExecutionNode{Step: haproxy.NewRestartHAProxyStep(ctx, restartNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableNode}})
	}

	return p, nil
}
