package coredns

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	dnsstep "github.com/mensylisir/kubexm/pkg/step/dns"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployCoreDNSTask struct {
	task.Base
}

func NewDeployCoreDNSTask() task.Task {
	return &DeployCoreDNSTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployCoreDNS",
				Description: "Deploy CoreDNS to the cluster",
			},
		},
	}
}

func (t *DeployCoreDNSTask) Name() string {
	return t.Meta.Name
}

func (t *DeployCoreDNSTask) Description() string {
	return t.Meta.Description
}

func (t *DeployCoreDNSTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *DeployCoreDNSTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy coredns")
	}
	executionHost := masterHosts[0]

	generateArtifacts := dnsstep.NewGenerateCoreDNSArtifactsStepBuilder(*runtimeCtx, "GenerateCoreDNSArtifacts").Build()
	installCoreDNS := dnsstep.NewInstallCoreDNSStepBuilder(*runtimeCtx, "InstallCoreDNS").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateCoreDNSArtifacts", Step: generateArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCoreDNS", Step: installCoreDNS, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateCoreDNSArtifacts", "InstallCoreDNS")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
