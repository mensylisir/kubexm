package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/loadbalancer"
)

type LoadBalancerModule struct {
	module.Base
}

func NewLoadBalancerModule(ctx *module.ModuleContext) (module.Interface, error) {
	s := &LoadBalancerModule{
		Base: module.Base{
			Name: "LoadBalancer",
			Desc: "Setup High Availability Load Balancer",
		},
	}

	haType := ctx.GetClusterConfig().Spec.HA.Type
	lbType := ctx.GetClusterConfig().Spec.HA.LoadBalancer.Type
	k8sType := ctx.GetClusterConfig().Spec.Kubernetes.Type

	var selectedTask task.Interface
	var err error

	if haType == "external" {
		selectedTask, err = loadbalancer.NewSetupExternalLoadBalancerTask(ctx)
	} else if haType == "internal" {
		if k8sType == "kubeadm" {
			selectedTask, err = loadbalancer.NewSetupInternalStaticPodLBTask(ctx)
		} else { // Assume binary
			selectedTask, err = loadbalancer.NewSetupInternalServiceLBTask(ctx)
		}
	} else if lbType == "kube-vip" {
		selectedTask, err = loadbalancer.NewSetupKubeVipTask(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer task: %w", err)
	}

	if selectedTask != nil {
		s.SetTasks([]task.Interface{selectedTask})
	}

	return s, nil
}

func (m *LoadBalancerModule) Execute(ctx *module.ModuleContext) (*plan.ExecutionGraph, error) {
	if len(m.GetTasks()) == 0 {
		// No LB configured, return empty graph
		return plan.NewExecutionGraph("Empty LoadBalancer Module"), nil
	}

	lbTask := m.GetTasks()[0]
	return lbTask.Execute(ctx)
}
