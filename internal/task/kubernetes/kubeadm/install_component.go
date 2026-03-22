package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubectl"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/internal/task"
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

	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to install kubernetes components")
	}

	installKubelet, err := kubelet.NewInstallKubeletStepBuilder(runtimeCtx, "InstallKubelet").Build()
	if err != nil {
		return nil, err
	}
	installKubeletSvc, err := kubelet.NewInstallKubeletServiceStepBuilder(runtimeCtx, "InstallKubeletService").Build()
	if err != nil {
		return nil, err
	}
	installKubeletDropin, err := kubelet.NewInstallKubeletDropInStepBuilder(runtimeCtx, "InstallKubeletDropin").Build()
	if err != nil {
		return nil, err
	}
	installKubeadm, err := kubeadm.NewInstallKubeadmStepBuilder(runtimeCtx, "InstallKubeadm").Build()
	if err != nil {
		return nil, err
	}
	installKubectl, err := kubectl.NewInstallKubectlStepBuilder(runtimeCtx, "InstallKubectl").Build()
	if err != nil {
		return nil, err
	}

	// Downloads are handled in Preflight PrepareAssets/ExtractBundle.
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubelet", Step: installKubelet, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeadm", Step: installKubeadm, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubectl", Step: installKubectl, Hosts: deployHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletService", Step: installKubeletSvc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletDropin", Step: installKubeletDropin, Hosts: deployHosts})

	fragment.AddDependency("InstallKubelet", "InstallKubeletService")
	fragment.AddDependency("InstallKubeletService", "InstallKubeletDropin")
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
