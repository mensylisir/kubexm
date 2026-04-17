package nodelocandns

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/dns"
	"github.com/mensylisir/kubexm/internal/task"
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
	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStepBuilder := dns.NewCleanNodeLocalDNSStepBuilder(runtimeCtx, "CleanNodeLocalDNSResources")

	cleanStep, err := cleanStepBuilder.Build()
	if err != nil {
		return nil, err
	}

	node := &plan.ExecutionNode{Name: "CleanNodeLocalDNSResources", Step: cleanStep, Hosts: []remotefw.Host{executionHost}}
	fragment.AddNode(node)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
