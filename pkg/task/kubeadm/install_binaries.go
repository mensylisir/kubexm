package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
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

		// These can all run in parallel for a given host.
		downloadKubeadm := kubeadm.NewDownloadKubeadmStep(ctx, fmt.Sprintf("DownloadKubeadm-%s", hostName))
		p.AddNode(plan.NodeID(downloadKubeadm.Meta().Name), &plan.ExecutionNode{Step: downloadKubeadm, Hosts: []connector.Host{host}})

		installKubeadm := kubeadm.NewInstallKubeadmStep(ctx, fmt.Sprintf("InstallKubeadm-%s", hostName))
		p.AddNode(plan.NodeID(installKubeadm.Meta().Name), &plan.ExecutionNode{Step: installKubeadm, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(downloadKubeadm.Meta().Name)}})

		downloadKubelet := kubelet.NewDownloadKubeletStep(ctx, fmt.Sprintf("DownloadKubelet-%s", hostName))
		p.AddNode(plan.NodeID(downloadKubelet.Meta().Name), &plan.ExecutionNode{Step: downloadKubelet, Hosts: []connector.Host{host}})

		installKubelet := kubelet.NewInstallKubeletStep(ctx, fmt.Sprintf("InstallKubelet-%s", hostName))
		p.AddNode(plan.NodeID(installKubelet.Meta().Name), &plan.ExecutionNode{Step: installKubelet, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(downloadKubelet.Meta().Name)}})

		downloadKubectl := kubectl.NewDownloadKubectlStep(ctx, fmt.Sprintf("DownloadKubectl-%s", hostName))
		p.AddNode(plan.NodeID(downloadKubectl.Meta().Name), &plan.ExecutionNode{Step: downloadKubectl, Hosts: []connector.Host{host}})

		installKubectl := kubectl.NewInstallKubectlStep(ctx, fmt.Sprintf("InstallKubectl-%s", hostName))
		p.AddNode(plan.NodeID(installKubectl.Meta().Name), &plan.ExecutionNode{Step: installKubectl, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(downloadKubectl.Meta().Name)}})

		// The service file for kubelet is also needed.
		installKubeletSvc := kubelet.NewInstallKubeletServiceStep(ctx, fmt.Sprintf("InstallKubeletSvc-%s", hostName))
		p.AddNode(plan.NodeID(installKubeletSvc.Meta().Name), &plan.ExecutionNode{Step: installKubeletSvc, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installKubelet.Meta().Name)}})
	}

	return p, nil
}
