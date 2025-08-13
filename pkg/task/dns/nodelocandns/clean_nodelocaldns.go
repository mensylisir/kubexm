package nodelocandns

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/dns"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanNodeLocalDNSTask struct {
	task.Base
}

func NewCleanNodeLocalDNSTask() task.Task {
	return &CleanNodeLocalDNSTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanNodeLocalDNS",
				Description: "Uninstall NodeLocal DNSCache addon and cleanup related resources",
			},
		},
	}
}

func (t *CleanNodeLocalDNSTask) Name() string {
	return t.Meta.Name
}

func (t *CleanNodeLocalDNSTask) Description() string {
	return t.Meta.Description
}

func (t *CleanNodeLocalDNSTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.DNS == nil || ctx.GetClusterConfig().Spec.DNS.NodeLocalDNS == nil {
		return false, nil
	}
	return *ctx.GetClusterConfig().Spec.DNS.NodeLocalDNS.Enabled, nil
}

func (t *CleanNodeLocalDNSTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStepBuilder := dns.NewCleanNodeLocalDNSStepBuilder(*runtimeCtx, "CleanNodeLocalDNSResources")

	cleanStep := cleanStepBuilder.Build()

	node := &plan.ExecutionNode{Name: "CleanNodeLocalDNSResources", Step: cleanStep, Hosts: []connector.Host{executionHost}}
	fragment.AddNode(node)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
