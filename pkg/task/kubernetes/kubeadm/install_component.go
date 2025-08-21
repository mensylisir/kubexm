package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubectl"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallKubeComponentsTask struct {
	task.Base
}

func NewInstallKubeComponentsTask() task.Task {
	return &InstallKubeComponentsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallKubeComponents",
				Description: "Install kubeadm, kubectl, and kubelet binaries and configure kubelet service on all nodes",
			},
		},
	}
}

func (t *InstallKubeComponentsTask) Name() string {
	return t.Meta.Name
}

func (t *InstallKubeComponentsTask) Description() string {
	return t.Meta.Description
}

func (t *InstallKubeComponentsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *InstallKubeComponentsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to install kubernetes components")
	}

	downloadKubelet := kubelet.NewDownloadKubeletStepBuilder(*runtimeCtx, "DownloadKubelet").Build()
	installKubelet := kubelet.NewInstallKubeletStepBuilder(*runtimeCtx, "InstallKubelet").Build()
	installKubeletSvc := kubelet.NewInstallKubeletServiceStepBuilder(*runtimeCtx, "InstallKubeletService").Build()
	installKubeletDropin := kubelet.NewInstallKubeletDropInStepBuilder(*runtimeCtx, "InstallKubeletDropin").Build()
	downloadKubeadm := kubeadm.NewDownloadKubeadmStepBuilder(*runtimeCtx, "DownloadKubeadm").Build()
	installKubeadm := kubeadm.NewInstallKubeadmStepBuilder(*runtimeCtx, "InstallKubeadm").Build()
	downloadKubectl := kubectl.NewDownloadKubectlStepBuilder(*runtimeCtx, "DownloadKubectl").Build()
	installKubectl := kubectl.NewInstallKubectlStepBuilder(*runtimeCtx, "InstallKubectl").Build()

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		ctx.GetLogger().Info("Online mode detected. Adding download steps for Kubernetes components.")
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadKubelet", Step: downloadKubelet, Hosts: []connector.Host{controlNode}})
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadKubeadm", Step: downloadKubeadm, Hosts: []connector.Host{controlNode}})
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadKubectl", Step: downloadKubectl, Hosts: []connector.Host{controlNode}})
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download steps for Kubernetes components.")
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubelet", Step: installKubelet, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeadm", Step: installKubeadm, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubectl", Step: installKubectl, Hosts: deployHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletService", Step: installKubeletSvc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletDropin", Step: installKubeletDropin, Hosts: deployHosts})

	if !isOffline {
		fragment.AddDependency("DownloadKubelet", "InstallKubelet")
		fragment.AddDependency("DownloadKubeadm", "InstallKubeadm")
		fragment.AddDependency("DownloadKubectl", "InstallKubectl")
	}

	fragment.AddDependency("InstallKubelet", "InstallKubeletService")
	fragment.AddDependency("InstallKubeletService", "InstallKubeletDropin")
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
