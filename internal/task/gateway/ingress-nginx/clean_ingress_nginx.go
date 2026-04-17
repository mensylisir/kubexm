package ingress_nginx

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/gateway/ingress-nginx"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanIngressNginxTask struct {
	task.Base
}

func NewCleanIngressNginxTask() task.Task {
	return &CleanIngressNginxTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanIngressNginx",
				Description: "Uninstall Ingress-Nginx controller and cleanup related resources",
			},
		},
	}
}

func (t *CleanIngressNginxTask) Name() string {
	return t.Meta.Name
}

func (t *CleanIngressNginxTask) Description() string {
	return t.Meta.Description
}

func (t *CleanIngressNginxTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Gateway == nil || ctx.GetClusterConfig().Spec.Gateway.IngressNginx == nil {
		return false, nil
	}
	if ctx.GetClusterConfig().Spec.Gateway.IngressNginx.Enabled == nil {
		return false, nil
	}
	return *ctx.GetClusterConfig().Spec.Gateway.IngressNginx.Enabled, nil
}

func (t *CleanIngressNginxTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep, err := ingressnginx.NewCleanIngressNginxStepBuilder(runtimeCtx, "UninstallIngressNginx").Build()
	if err != nil {
		return nil, err
	}

	node := &plan.ExecutionNode{Name: "UninstallIngressNginx", Step: cleanStep, Hosts: []remotefw.Host{executionHost}}
	fragment.AddNode(node)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
