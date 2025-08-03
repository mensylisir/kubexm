package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubectl"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallKubeadmBinariesTask struct {
	task.Base
}

func NewInstallKubeadmBinariesTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallKubeadmBinariesTask{
		Base: task.Base{
			Name:   "InstallKubeadmBinaries",
			Desc:   "Install kubeadm, kubelet, and kubectl binaries on all nodes",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallKubeadmBinariesAction),
		},
	}
	return s, nil
}

type InstallKubeadmBinariesAction struct {
	task.Action
}

func (a *InstallKubeadmBinariesAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Kubeadm Binaries Phase")

	hosts := a.GetHosts()
	if len(hosts) == 0 {
		return p, nil
	}

	for _, host := range hosts {
		hostName := host.GetName()

		dlKubeadmNode := plan.NodeID(fmt.Sprintf("download-kubeadm-%s", hostName))
		p.AddNode(dlKubeadmNode, &plan.ExecutionNode{Step: kubeadm.NewDownloadKubeadmStep(ctx, dlKubeadmNode.String()), Hosts: []connector.Host{host}})

		installKubeadmNode := plan.NodeID(fmt.Sprintf("install-kubeadm-%s", hostName))
		p.AddNode(installKubeadmNode, &plan.ExecutionNode{Step: kubeadm.NewInstallKubeadmStep(ctx, installKubeadmNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{dlKubeadmNode}})

		dlKubeletNode := plan.NodeID(fmt.Sprintf("download-kubelet-%s", hostName))
		p.AddNode(dlKubeletNode, &plan.ExecutionNode{Step: kubelet.NewDownloadKubeletStep(ctx, dlKubeletNode.String()), Hosts: []connector.Host{host}})

		installKubeletNode := plan.NodeID(fmt.Sprintf("install-kubelet-%s", hostName))
		p.AddNode(installKubeletNode, &plan.ExecutionNode{Step: kubelet.NewInstallKubeletStep(ctx, installKubeletNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{dlKubeletNode}})

		dlKubectlNode := plan.NodeID(fmt.Sprintf("download-kubectl-%s", hostName))
		p.AddNode(dlKubectlNode, &plan.ExecutionNode{Step: kubectl.NewDownloadKubectlStep(ctx, dlKubectlNode.String()), Hosts: []connector.Host{host}})

		installKubectlNode := plan.NodeID(fmt.Sprintf("install-kubectl-%s", hostName))
		p.AddNode(installKubectlNode, &plan.ExecutionNode{Step: kubectl.NewInstallKubectlStep(ctx, installKubectlNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{dlKubectlNode}})

		installKubeletSvcNode := plan.NodeID(fmt.Sprintf("install-kubelet-svc-%s", hostName))
		p.AddNode(installKubeletSvcNode, &plan.ExecutionNode{Step: kubelet.NewInstallKubeletServiceStep(ctx, installKubeletSvcNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installKubeletNode}})
	}

	return p, nil
}
