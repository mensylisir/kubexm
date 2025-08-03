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

	for _, host := range hosts {
		hostName := host.GetName()

		components := map[string]struct{download plan.Step; install plan.Step}{
			"kube-apiserver":          {apiserver.NewDownloadApiServerStep(ctx, "..."), apiserver.NewInstallApiServerStep(ctx, "...")},
			"kube-controller-manager": {controllermanager.NewDownloadControllerManagerStep(ctx, "..."), controllermanager.NewInstallControllerManagerStep(ctx, "...")},
			"kube-scheduler":          {scheduler.NewDownloadSchedulerStep(ctx, "..."), scheduler.NewInstallSchedulerStep(ctx, "...")},
			"kube-proxy":              {kubeproxy.NewDownloadKubeProxyStep(ctx, "..."), kubeproxy.NewInstallKubeProxyStep(ctx, "...")},
			"kubelet":                 {kubelet.NewDownloadKubeletStep(ctx, "..."), kubelet.NewInstallKubeletStep(ctx, "...")},
			"kubectl":                 {kubectl.NewDownloadKubectlStep(ctx, "..."), kubectl.NewInstallKubectlStep(ctx, "...")},
		}

		for name, steps := range components {
			downloadNodeID := plan.NodeID(fmt.Sprintf("download-%s-%s", name, hostName))
			installNodeID := plan.NodeID(fmt.Sprintf("install-%s-%s", name, hostName))

			// This is not quite right, as the step builders need unique names.
			// The previous implementation was better. I will revert to that pattern.
			// I am correcting my own mistake here.

			var downloadStep, installStep plan.Step
			switch name {
			case "kube-apiserver":
				downloadStep = apiserver.NewDownloadApiServerStep(ctx, downloadNodeID.String())
				installStep = apiserver.NewInstallApiServerStep(ctx, installNodeID.String())
			case "kube-controller-manager":
				downloadStep = controllermanager.NewDownloadControllerManagerStep(ctx, downloadNodeID.String())
				installStep = controllermanager.NewInstallControllerManagerStep(ctx, installNodeID.String())
			case "kube-scheduler":
				downloadStep = scheduler.NewDownloadSchedulerStep(ctx, downloadNodeID.String())
				installStep = scheduler.NewInstallSchedulerStep(ctx, installNodeID.String())
			case "kube-proxy":
				downloadStep = kubeproxy.NewDownloadKubeProxyStep(ctx, downloadNodeID.String())
				installStep = kubeproxy.NewInstallKubeProxyStep(ctx, installNodeID.String())
			case "kubelet":
				downloadStep = kubelet.NewDownloadKubeletStep(ctx, downloadNodeID.String())
				installStep = kubelet.NewInstallKubeletStep(ctx, installNodeID.String())
			case "kubectl":
				downloadStep = kubectl.NewDownloadKubectlStep(ctx, downloadNodeID.String())
				installStep = kubectl.NewInstallKubectlStep(ctx, installNodeID.String())
			}

			p.AddNode(downloadNodeID, &plan.ExecutionNode{Step: downloadStep, Hosts: []connector.Host{host}})
			p.AddNode(installNodeID, &plan.ExecutionNode{Step: installStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{downloadNodeID}})
		}
	}

	return p, nil
}
