package coredns

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	dnsstep "github.com/mensylisir/kubexm/internal/step/dns"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy coredns")
	}
	executionHost := masterHosts[0]

	generateArtifacts, err := dnsstep.NewGenerateCoreDNSArtifactsStepBuilder(runtimeCtx, "GenerateCoreDNSArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installCoreDNS, err := dnsstep.NewInstallCoreDNSStepBuilder(runtimeCtx, "InstallCoreDNS").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateCoreDNSArtifacts", Step: generateArtifacts, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCoreDNS", Step: installCoreDNS, Hosts: []remotefw.Host{executionHost}})

	fragment.AddDependency("GenerateCoreDNSArtifacts", "InstallCoreDNS")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
