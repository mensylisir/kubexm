package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/health"
	"github.com/mensylisir/kubexm/internal/task"
)

// ===================================================================
// Health Check Tasks - 基于已有的 step 实现
// ===================================================================

// CheckAPIServerHealthTask 检查 kube-apiserver 健康状态
type CheckAPIServerHealthTask struct {
	task.Base
}

func NewCheckAPIServerHealthTask() task.Task {
	return &CheckAPIServerHealthTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckAPIServerHealth",
				Description: "Check kube-apiserver health status on all control plane nodes",
			},
		},
	}
}

func (t *CheckAPIServerHealthTask) Name() string        { return t.Meta.Name }
func (t *CheckAPIServerHealthTask) Description() string { return t.Meta.Description }

func (t *CheckAPIServerHealthTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *CheckAPIServerHealthTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		step, err := health.NewVerifyAPIServerHealthStepBuilder(
			runtime.ForHost(execCtx, host), "VerifyAPIServerHealth").Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create apiserver health step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("VerifyAPIServerHealth-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// CheckSchedulerHealthTask 检查 kube-scheduler 健康状态
type CheckSchedulerHealthTask struct {
	task.Base
}

func NewCheckSchedulerHealthTask() task.Task {
	return &CheckSchedulerHealthTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckSchedulerHealth",
				Description: "Check kube-scheduler health status on all control plane nodes",
			},
		},
	}
}

func (t *CheckSchedulerHealthTask) Name() string        { return t.Meta.Name }
func (t *CheckSchedulerHealthTask) Description() string { return t.Meta.Description }

func (t *CheckSchedulerHealthTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *CheckSchedulerHealthTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		step, err := health.NewVerifySchedulerHealthStepBuilder(
			runtime.ForHost(execCtx, host), "VerifySchedulerHealth").Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create scheduler health step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("VerifySchedulerHealth-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// CheckControllerManagerHealthTask 检查 kube-controller-manager 健康状态
type CheckControllerManagerHealthTask struct {
	task.Base
}

func NewCheckControllerManagerHealthTask() task.Task {
	return &CheckControllerManagerHealthTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckControllerManagerHealth",
				Description: "Check kube-controller-manager health status on all control plane nodes",
			},
		},
	}
}

func (t *CheckControllerManagerHealthTask) Name() string        { return t.Meta.Name }
func (t *CheckControllerManagerHealthTask) Description() string { return t.Meta.Description }

func (t *CheckControllerManagerHealthTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *CheckControllerManagerHealthTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		step, err := health.NewVerifyControllerManagerHealthStepBuilder(
			runtime.ForHost(execCtx, host), "VerifyControllerManagerHealth").Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create controller-manager health step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("VerifyControllerManagerHealth-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// CheckKubeletHealthTask 检查 kubelet 健康状态
type CheckKubeletHealthTask struct {
	task.Base
}

func NewCheckKubeletHealthTask() task.Task {
	return &CheckKubeletHealthTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckKubeletHealth",
				Description: "Check kubelet health status on all nodes",
			},
		},
	}
}

func (t *CheckKubeletHealthTask) Name() string        { return t.Meta.Name }
func (t *CheckKubeletHealthTask) Description() string { return t.Meta.Description }

func (t *CheckKubeletHealthTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cpHosts := ctx.GetHostsByRole(common.RoleControlPlane)
	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	return len(cpHosts)+len(workerHosts) > 0, nil
}

func (t *CheckKubeletHealthTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())

	cpHosts := ctx.GetHostsByRole(common.RoleControlPlane)
	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	allHosts := append(cpHosts, workerHosts...)

	if len(allHosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range allHosts {
		step, err := health.NewVerifyKubeletHealthStepBuilder(
			runtime.ForHost(execCtx, host), "VerifyKubeletHealth").Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubelet health step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("VerifyKubeletHealth-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// CheckClusterHealthTask 检查整个集群健康状态（节点+Pod+etcd）
type CheckClusterHealthTask struct {
	task.Base
}

func NewCheckClusterHealthTask() task.Task {
	return &CheckClusterHealthTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckClusterHealth",
				Description: "Check overall cluster health (nodes, pods, etcd) from control plane",
			},
		},
	}
}

func (t *CheckClusterHealthTask) Name() string        { return t.Meta.Name }
func (t *CheckClusterHealthTask) Description() string { return t.Meta.Description }

func (t *CheckClusterHealthTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	return len(hosts) > 0, nil
}

func (t *CheckClusterHealthTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	host := hosts[0]
	step, err := health.NewCheckClusterHealthStepBuilder(
		runtime.ForHost(execCtx, host), "CheckClusterHealth").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster health step: %w", err)
	}

	nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  "CheckClusterHealth",
		Step:  step,
		Hosts: []remotefw.Host{host},
	})
	fragment.EntryNodes = []plan.NodeID{nodeID}
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
