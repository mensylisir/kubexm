package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubectl"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallKubernetesBinariesTask struct {
	task.Base
}

func NewInstallKubernetesBinariesTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallKubernetesBinariesTask{
		Base: task.Base{
			Name:   "InstallKubernetesBinaries",
			Desc:   "Download and install all Kubernetes binaries for a from-scratch installation",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallKubernetesBinariesAction),
		},
	}
	return s, nil
}

type InstallKubernetesBinariesAction struct {
	task.Action
}

func (a *InstallKubernetesBinariesAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Kubernetes Binaries Phase")

	hosts := a.GetHosts()
	if len(hosts) == 0 {
		return p, nil
	}

	components := []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler", "kube-proxy", "kubelet", "kubectl"}

	for _, host := range hosts {
		hostName := host.GetName()

		for _, component := range components {
			var downloadStep, installStep plan.Step
			var err error

			// Using a factory pattern here would be cleaner, but for now, a switch will do.
			switch component {
			case "kube-apiserver":
				downloadStep = apiserver.NewDownloadApiServerStep(ctx, fmt.Sprintf("DownloadApiServer-%s", hostName))
				installStep = apiserver.NewInstallApiServerStep(ctx, fmt.Sprintf("InstallApiServer-%s", hostName))
			case "kube-controller-manager":
				downloadStep = controllermanager.NewDownloadControllerManagerStep(ctx, fmt.Sprintf("DownloadControllerManager-%s", hostName))
				installStep = controllermanager.NewInstallControllerManagerStep(ctx, fmt.Sprintf("InstallControllerManager-%s", hostName))
			case "kube-scheduler":
				downloadStep = scheduler.NewDownloadSchedulerStep(ctx, fmt.Sprintf("DownloadScheduler-%s", hostName))
				installStep = scheduler.NewInstallSchedulerStep(ctx, fmt.Sprintf("InstallScheduler-%s", hostName))
			case "kube-proxy":
				downloadStep = kubeproxy.NewDownloadKubeProxyStep(ctx, fmt.Sprintf("DownloadKubeProxy-%s", hostName))
				installStep = kubeproxy.NewInstallKubeProxyStep(ctx, fmt.Sprintf("InstallKubeProxy-%s", hostName))
			case "kubelet":
				downloadStep = kubelet.NewDownloadKubeletStep(ctx, fmt.Sprintf("DownloadKubelet-%s", hostName))
				installStep = kubelet.NewInstallKubeletStep(ctx, fmt.Sprintf("InstallKubelet-%s", hostName))
			case "kubectl":
				downloadStep = kubectl.NewDownloadKubectlStep(ctx, fmt.Sprintf("DownloadKubectl-%s", hostName))
				installStep = kubectl.NewInstallKubectlStep(ctx, fmt.Sprintf("InstallKubectl-%s", hostName))
			default:
				return nil, fmt.Errorf("unknown kubernetes binary component: %s", component)
			}

			if err != nil {
				return nil, err
			}

			downloadNodeID := plan.NodeID(downloadStep.Meta().Name)
			installNodeID := plan.NodeID(installStep.Meta().Name)

			p.AddNode(downloadNodeID, &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{host}})
			p.AddNode(installNodeID, &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{downloadNodeID}})
		}
	}

	return p, nil
}
