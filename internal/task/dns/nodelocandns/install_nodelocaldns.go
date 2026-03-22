package nodelocandns

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/dns"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployNodeLocalDNSTask struct {
	task.Base
}

func NewDeployNodeLocalDNSTask() task.Task {
	return &DeployNodeLocalDNSTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNodeLocalDNS",
				Description: "Deploy NodeLocal DNSCache addon to improve DNS performance",
			},
		},
	}
}

func (t *DeployNodeLocalDNSTask) Name() string {
	return t.Meta.Name
}

func (t *DeployNodeLocalDNSTask) Description() string {
	return t.Meta.Description
}

func (t *DeployNodeLocalDNSTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.DNS == nil || ctx.GetClusterConfig().Spec.DNS.NodeLocalDNS == nil {
		return false, nil
	}
	return *ctx.GetClusterConfig().Spec.DNS.NodeLocalDNS.Enabled, nil
}

func (t *DeployNodeLocalDNSTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy NodeLocal DNSCache")
	}
	executionHost := masterHosts[0]

	generateManifests, err := dns.NewGenerateNodeLocalDNSArtifactsStepBuilder(runtimeCtx, "GenerateNodeLocalDNSManifests").Build()
	if err != nil {
		return nil, err
	}
	installNodeLocalDNS, err := dns.NewInstallNodeLocalDNSStepBuilder(runtimeCtx, "InstallNodeLocalDNS").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNodeLocalDNSManifests", Step: generateManifests, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNodeLocalDNS", Step: installNodeLocalDNS, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateNodeLocalDNSManifests", "InstallNodeLocalDNS")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
