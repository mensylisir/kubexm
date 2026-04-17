package reconfigure

import (
	"fmt"

	k8scommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/common"
	kubeapiserver "github.com/mensylisir/kubexm/internal/step/kubernetes/apiserver"
	kubecontroller "github.com/mensylisir/kubexm/internal/step/kubernetes/controller-manager"
	kubelet "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	kubeproxy "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	kubescheduler "github.com/mensylisir/kubexm/internal/step/kubernetes/scheduler"
	"github.com/mensylisir/kubexm/internal/task"
)

// ===================================================================
// Reconfigure Tasks - reconfigure cluster components
// ===================================================================

// ReconfigureAPIServerTask reconfigures kube-apiserver on all control plane nodes.
type ReconfigureAPIServerTask struct {
	task.Base
}

func NewReconfigureAPIServerTask() task.Task {
	return &ReconfigureAPIServerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ReconfigureAPIServer",
				Description: "Reconfigure kube-apiserver on all control plane nodes",
			},
		},
	}
}

func (t *ReconfigureAPIServerTask) Name() string        { return t.Meta.Name }
func (t *ReconfigureAPIServerTask) Description() string { return t.Meta.Description }

func (t *ReconfigureAPIServerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *ReconfigureAPIServerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		// Step 1: Configure apiserver
		configureStep, err := kubeapiserver.NewConfigureKubeAPIServerStepBuilder(
			hostCtx, fmt.Sprintf("ConfigureAPIServer-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create apiserver configure step for %s: %w", host.GetName(), err)
		}
		configNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("ConfigureAPIServer-%s", host.GetName()),
			Step:  configureStep,
			Hosts: []remotefw.Host{host},
		})

		// Step 2: Restart apiserver service
		restartStep, err := common.NewManageServiceStepBuilder(
			hostCtx, fmt.Sprintf("RestartAPIServer-%s", host.GetName()), "kube-apiserver", common.ActionRestart).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create apiserver restart step for %s: %w", host.GetName(), err)
		}
		restartNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestartAPIServer-%s", host.GetName()),
			Step:  restartStep,
			Hosts: []remotefw.Host{host},
		})

		// Link configure -> restart
		fragment.AddDependency(configNodeID, restartNodeID)
		entryNodes = append(entryNodes, configNodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// ReconfigureSchedulerTask reconfigures kube-scheduler on all control plane nodes.
type ReconfigureSchedulerTask struct {
	task.Base
}

func NewReconfigureSchedulerTask() task.Task {
	return &ReconfigureSchedulerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ReconfigureScheduler",
				Description: "Reconfigure kube-scheduler on all control plane nodes",
			},
		},
	}
}

func (t *ReconfigureSchedulerTask) Name() string        { return t.Meta.Name }
func (t *ReconfigureSchedulerTask) Description() string { return t.Meta.Description }

func (t *ReconfigureSchedulerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *ReconfigureSchedulerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		configureStep, err := kubescheduler.NewConfigureKubeSchedulerStepBuilder(
			hostCtx, fmt.Sprintf("ConfigureScheduler-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create scheduler configure step for %s: %w", host.GetName(), err)
		}
		configNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("ConfigureScheduler-%s", host.GetName()),
			Step:  configureStep,
			Hosts: []remotefw.Host{host},
		})

		restartStep, err := common.NewManageServiceStepBuilder(
			hostCtx, fmt.Sprintf("RestartScheduler-%s", host.GetName()), "kube-scheduler", common.ActionRestart).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create scheduler restart step for %s: %w", host.GetName(), err)
		}
		restartNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestartScheduler-%s", host.GetName()),
			Step:  restartStep,
			Hosts: []remotefw.Host{host},
		})

		fragment.AddDependency(configNodeID, restartNodeID)
		entryNodes = append(entryNodes, configNodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// ReconfigureControllerManagerTask reconfigures kube-controller-manager on all control plane nodes.
type ReconfigureControllerManagerTask struct {
	task.Base
}

func NewReconfigureControllerManagerTask() task.Task {
	return &ReconfigureControllerManagerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ReconfigureControllerManager",
				Description: "Reconfigure kube-controller-manager on all control plane nodes",
			},
		},
	}
}

func (t *ReconfigureControllerManagerTask) Name() string        { return t.Meta.Name }
func (t *ReconfigureControllerManagerTask) Description() string { return t.Meta.Description }

func (t *ReconfigureControllerManagerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *ReconfigureControllerManagerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		configureStep, err := kubecontroller.NewConfigureKubeControllerManagerStepBuilder(
			hostCtx, fmt.Sprintf("ConfigureControllerManager-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create controller-manager configure step for %s: %w", host.GetName(), err)
		}
		configNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("ConfigureControllerManager-%s", host.GetName()),
			Step:  configureStep,
			Hosts: []remotefw.Host{host},
		})

		restartStep, err := common.NewManageServiceStepBuilder(
			hostCtx, fmt.Sprintf("RestartControllerManager-%s", host.GetName()), "kube-controller-manager", common.ActionRestart).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create controller-manager restart step for %s: %w", host.GetName(), err)
		}
		restartNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestartControllerManager-%s", host.GetName()),
			Step:  restartStep,
			Hosts: []remotefw.Host{host},
		})

		fragment.AddDependency(configNodeID, restartNodeID)
		entryNodes = append(entryNodes, configNodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// ReconfigureKubeletTask reconfigures kubelet on all nodes.
type ReconfigureKubeletTask struct {
	task.Base
}

func NewReconfigureKubeletTask() task.Task {
	return &ReconfigureKubeletTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ReconfigureKubelet",
				Description: "Reconfigure kubelet on all nodes",
			},
		},
	}
}

func (t *ReconfigureKubeletTask) Name() string        { return t.Meta.Name }
func (t *ReconfigureKubeletTask) Description() string { return t.Meta.Description }

func (t *ReconfigureKubeletTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cpHosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	workerHosts := ctx.GetHostsByRole(k8scommon.RoleWorker)
	return len(cpHosts)+len(workerHosts) > 0, nil
}

func (t *ReconfigureKubeletTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())

	cpHosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	workerHosts := ctx.GetHostsByRole(k8scommon.RoleWorker)
	allHosts := append(cpHosts, workerHosts...)
	if len(allHosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range allHosts {
		hostCtx := runtime.ForHost(execCtx, host)

		configStep, err := kubelet.NewCreateKubeletConfigYAMLStepBuilder(
			hostCtx, fmt.Sprintf("GenerateKubeletConfig-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubelet config step for %s: %w", host.GetName(), err)
		}
		configNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("GenerateKubeletConfig-%s", host.GetName()),
			Step:  configStep,
			Hosts: []remotefw.Host{host},
		})

		restartStep, err := common.NewManageServiceStepBuilder(
			hostCtx, fmt.Sprintf("RestartKubelet-%s", host.GetName()), "kubelet", common.ActionRestart).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubelet restart step for %s: %w", host.GetName(), err)
		}
		restartNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestartKubelet-%s", host.GetName()),
			Step:  restartStep,
			Hosts: []remotefw.Host{host},
		})

		fragment.AddDependency(configNodeID, restartNodeID)
		entryNodes = append(entryNodes, configNodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// ReconfigureProxyTask reconfigures kube-proxy on all nodes.
type ReconfigureProxyTask struct {
	task.Base
}

func NewReconfigureProxyTask() task.Task {
	return &ReconfigureProxyTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ReconfigureProxy",
				Description: "Reconfigure kube-proxy on all nodes",
			},
		},
	}
}

func (t *ReconfigureProxyTask) Name() string        { return t.Meta.Name }
func (t *ReconfigureProxyTask) Description() string { return t.Meta.Description }

func (t *ReconfigureProxyTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cpHosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	workerHosts := ctx.GetHostsByRole(k8scommon.RoleWorker)
	return len(cpHosts)+len(workerHosts) > 0, nil
}

func (t *ReconfigureProxyTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())

	cpHosts := ctx.GetHostsByRole(k8scommon.RoleControlPlane)
	workerHosts := ctx.GetHostsByRole(k8scommon.RoleWorker)
	allHosts := append(cpHosts, workerHosts...)
	if len(allHosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range allHosts {
		hostCtx := runtime.ForHost(execCtx, host)

		configStep, err := kubeproxy.NewCreateKubeProxyConfigYAMLStepBuilder(
			hostCtx, fmt.Sprintf("GenerateProxyConfig-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy config step for %s: %w", host.GetName(), err)
		}
		configNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("GenerateProxyConfig-%s", host.GetName()),
			Step:  configStep,
			Hosts: []remotefw.Host{host},
		})

		restartStep, err := common.NewManageServiceStepBuilder(
			hostCtx, fmt.Sprintf("RestartProxy-%s", host.GetName()), "kube-proxy", common.ActionRestart).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy restart step for %s: %w", host.GetName(), err)
		}
		restartNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestartProxy-%s", host.GetName()),
			Step:  restartStep,
			Hosts: []remotefw.Host{host},
		})

		fragment.AddDependency(configNodeID, restartNodeID)
		entryNodes = append(entryNodes, configNodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
