package coredns

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	dnsstep "github.com/mensylisir/kubexm/pkg/step/dns"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanCoreDNSTask struct {
	task.Base
}

func NewCleanCoreDNSTask() task.Task {
	return &CleanCoreDNSTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanCoreDNS",
				Description: "Clean up CoreDNS resources from the cluster",
			},
		},
	}
}

func (t *CleanCoreDNSTask) Name() string {
	return t.Meta.Name
}

func (t *CleanCoreDNSTask) Description() string {
	return t.Meta.Description
}

func (t *CleanCoreDNSTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanCoreDNSTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep := dnsstep.NewCleanCoreDNSStepBuilder(*runtimeCtx, "CleanCoreDNSResources").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanCoreDNSResources", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
