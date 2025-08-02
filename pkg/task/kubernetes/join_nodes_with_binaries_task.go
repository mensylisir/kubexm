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
		// Assumes kubelet binary is already installed from a previous task.
		// Also assumes certs and kubeconfigs have been generated and distributed.
		createKubeletConfig := kubelet.NewCreateKubeletConfigStep(ctx, fmt.Sprintf("CreateKubeletConfig-%s", hostName))
		p.AddNode(plan.NodeID(createKubeletConfig.Meta().Name), &plan.ExecutionNode{Step: createKubeletConfig, Hosts: []connector.Host{host}})

		installKubeletSvc := kubelet.NewInstallKubeletServiceStep(ctx, fmt.Sprintf("InstallKubeletService-%s", hostName))
		p.AddNode(plan.NodeID(installKubeletSvc.Meta().Name), &plan.ExecutionNode{Step: installKubeletSvc, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(createKubeletConfig.Meta().Name)}})

		enableKubelet := kubelet.NewEnableKubeletServiceStep(ctx, fmt.Sprintf("EnableKubelet-%s", hostName))
		p.AddNode(plan.NodeID(enableKubelet.Meta().Name), &plan.ExecutionNode{Step: enableKubelet, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installKubeletSvc.Meta().Name)}})

		startKubelet := kubelet.NewStartKubeletStep(ctx, fmt.Sprintf("StartKubelet-%s", hostName))
		p.AddNode(plan.NodeID(startKubelet.Meta().Name), &plan.ExecutionNode{Step: startKubelet, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(enableKubelet.Meta().Name)}})
		kubeletReadyNode := plan.NodeID(startKubelet.Meta().Name)

		// --- Kube-proxy ---
		genProxyKubeconfig := kubeproxy.NewGenerateKubeProxyKubeconfigStep(ctx, fmt.Sprintf("GenerateKubeProxyKubeconfig-%s", hostName))
		p.AddNode(plan.NodeID(genProxyKubeconfig.Meta().Name), &plan.ExecutionNode{Step: genProxyKubeconfig, Hosts: []connector.Host{host}})

		genProxyConfig := kubeproxy.NewGenerateKubeProxyConfigStep(ctx, fmt.Sprintf("GenerateKubeProxyConfig-%s", hostName))
		p.AddNode(plan.NodeID(genProxyConfig.Meta().Name), &plan.ExecutionNode{Step: genProxyConfig, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(genProxyKubeconfig.Meta().Name)}})

		installProxySvc := kubeproxy.NewInstallKubeProxyServiceStep(ctx, fmt.Sprintf("InstallKubeProxyService-%s", hostName))
		p.AddNode(plan.NodeID(installProxySvc.Meta().Name), &plan.ExecutionNode{Step: installProxySvc, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(genProxyConfig.Meta().Name)}})

		enableProxy := kubeproxy.NewEnableKubeProxyStep(ctx, fmt.Sprintf("EnableKubeProxy-%s", hostName))
		p.AddNode(plan.NodeID(enableProxy.Meta().Name), &plan.ExecutionNode{Step: enableProxy, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installProxySvc.Meta().Name)}})

		startProxy := kubeproxy.NewStartKubeProxyStep(ctx, fmt.Sprintf("StartKubeProxy-%s", hostName))
		p.AddNode(plan.NodeID(startProxy.Meta().Name), &plan.ExecutionNode{Step: startProxy, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(enableProxy.Meta().Name), kubeletReadyNode}})
	}

	return p, nil
}
