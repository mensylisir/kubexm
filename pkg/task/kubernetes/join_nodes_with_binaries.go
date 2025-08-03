package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type JoinNodesWithBinariesTask struct {
	task.Base
}

func NewJoinNodesWithBinariesTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &JoinNodesWithBinariesTask{
		Base: task.Base{
			Name:   "JoinNodesWithBinaries",
			Desc:   "Configure and start kubelet and kube-proxy on worker nodes",
			Hosts:  ctx.GetHostsByRole(common.RoleWorker),
			Action: new(JoinNodesWithBinariesAction),
		},
	}
	return s, nil
}

type JoinNodesWithBinariesAction struct {
	task.Action
}

func (a *JoinNodesWithBinariesAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Join Nodes (Binary) Phase")

	workerHosts := a.GetHosts()

	for _, host := range workerHosts {
		hostName := host.GetName()

		// --- Kubelet ---
		createKubeletCfgNode := plan.NodeID(fmt.Sprintf("create-kubelet-cfg-%s", hostName))
		p.AddNode(createKubeletCfgNode, &plan.ExecutionNode{Step: kubelet.NewCreateKubeletConfigStep(ctx, createKubeletCfgNode.String()), Hosts: []connector.Host{host}})

		installKubeletSvcNode := plan.NodeID(fmt.Sprintf("install-kubelet-svc-%s", hostName))
		p.AddNode(installKubeletSvcNode, &plan.ExecutionNode{Step: kubelet.NewInstallKubeletServiceStep(ctx, installKubeletSvcNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{createKubeletCfgNode}})

		enableKubeletNode := plan.NodeID(fmt.Sprintf("enable-kubelet-%s", hostName))
		p.AddNode(enableKubeletNode, &plan.ExecutionNode{Step: kubelet.NewEnableKubeletServiceStep(ctx, enableKubeletNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installKubeletSvcNode}})

		startKubeletNode := plan.NodeID(fmt.Sprintf("start-kubelet-%s", hostName))
		p.AddNode(startKubeletNode, &plan.ExecutionNode{Step: kubelet.NewStartKubeletStep(ctx, startKubeletNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableKubeletNode}})

		// --- Kube-proxy ---
		genProxyKubeconfigNode := plan.NodeID(fmt.Sprintf("gen-proxy-kubeconfig-%s", hostName))
		p.AddNode(genProxyKubeconfigNode, &plan.ExecutionNode{Step: kubeproxy.NewGenerateKubeProxyKubeconfigStep(ctx, genProxyKubeconfigNode.String()), Hosts: []connector.Host{host}})

		genProxyCfgNode := plan.NodeID(fmt.Sprintf("gen-proxy-cfg-%s", hostName))
		p.AddNode(genProxyCfgNode, &plan.ExecutionNode{Step: kubeproxy.NewGenerateKubeProxyConfigStep(ctx, genProxyCfgNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genProxyKubeconfigNode}})

		installProxySvcNode := plan.NodeID(fmt.Sprintf("install-proxy-svc-%s", hostName))
		p.AddNode(installProxySvcNode, &plan.ExecutionNode{Step: kubeproxy.NewInstallKubeProxyServiceStep(ctx, installProxySvcNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genProxyCfgNode}})

		enableProxyNode := plan.NodeID(fmt.Sprintf("enable-proxy-%s", hostName))
		p.AddNode(enableProxyNode, &plan.ExecutionNode{Step: kubeproxy.NewEnableKubeProxyStep(ctx, enableProxyNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installProxySvcNode}})

		startProxyNode := plan.NodeID(fmt.Sprintf("start-proxy-%s", hostName))
		p.AddNode(startProxyNode, &plan.ExecutionNode{Step: kubeproxy.NewStartKubeProxyStep(ctx, startProxyNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableProxyNode, startKubeletNode}})
	}

	return p, nil
}
