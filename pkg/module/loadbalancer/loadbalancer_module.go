package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/loadbalancer"
)

// DeployLoadBalancerTask orchestrates load balancer deployment
type DeployLoadBalancerTask struct {
	task.Base
}

func NewDeployLoadBalancerTask() task.Task {
	return &DeployLoadBalancerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployLoadBalancer",
				Description: "Deploys load balancer based on cluster configuration",
			},
		},
	}
}

func (t *DeployLoadBalancerTask) Name() string {
	return t.Meta.Name
}

func (t *DeployLoadBalancerTask) Description() string {
	return t.Meta.Description
}

func (t *DeployLoadBalancerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg == nil {
		return false, nil
	}
	cpEndpoint := cfg.Spec.ControlPlaneEndpoint
	if cpEndpoint == nil || cpEndpoint.HighAvailability == nil {
		return false, nil
	}
	if cpEndpoint.HighAvailability.Enabled == nil {
		return false, nil
	}
	return *cpEndpoint.HighAvailability.Enabled, nil
}

func (t *DeployLoadBalancerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	tasks := loadbalancer.GetLoadBalancerTasks(ctx)

	var previousTaskExitNodes []plan.NodeID

	for _, lbTask := range tasks {
		taskFrag, err := lbTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan load balancer task %s: %w", lbTask.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}
		if err := fragment.MergeFragment(taskFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(fragment, previousTaskExitNodes, taskFrag.EntryNodes)
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(previousTaskExitNodes) == 0 {
		return plan.NewEmptyFragment(t.Name()), nil
	}

	return fragment, nil
}

func (t *DeployLoadBalancerTask) GetBase() *task.Base {
	return &t.Base
}

// LoadBalancerModule orchestrates load balancer deployment
type LoadBalancerModule struct {
	module.BaseModule
}

func NewLoadBalancerModule() module.Module {
	tasks := []task.Task{
		NewDeployLoadBalancerTask(),
	}
	base := module.NewBaseModule("LoadBalancerSetup", tasks)
	return &LoadBalancerModule{BaseModule: base}
}

func (m *LoadBalancerModule) Name() string {
	return "LoadBalancerSetup"
}

func (m *LoadBalancerModule) Description() string {
	return "Deploys load balancer (Internal/External/Haproxy/Nginx/Kube-VIP) based on cluster configuration"
}

func (m *LoadBalancerModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		logger.Info("No cluster config found, skipping load balancer")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	cpEndpoint := clusterCfg.Spec.ControlPlaneEndpoint
	if cpEndpoint == nil || cpEndpoint.HighAvailability == nil || cpEndpoint.HighAvailability.Enabled == nil || !*cpEndpoint.HighAvailability.Enabled {
		logger.Info("Load balancer is disabled, skipping")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	lbTask := NewDeployLoadBalancerTask()
	lbFrag, err := lbTask.Plan(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan load balancer task: %w", err)
	}

	if lbFrag.IsEmpty() {
		logger.Info("Load balancer task returned empty fragment, skipping")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	if err := moduleFragment.MergeFragment(lbFrag); err != nil {
		return nil, err
	}

	moduleFragment.EntryNodes = lbFrag.EntryNodes
	moduleFragment.ExitNodes = lbFrag.ExitNodes

	return moduleFragment, nil
}

func (m *LoadBalancerModule) GetBase() *module.BaseModule {
	return &m.BaseModule
}
