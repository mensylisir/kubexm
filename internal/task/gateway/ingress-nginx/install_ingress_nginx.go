package ingress_nginx

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	ingressnginx "github.com/mensylisir/kubexm/internal/step/gateway/ingress-nginx"

	"github.com/mensylisir/kubexm/internal/task"
)

type DeployIngressNginxTask struct {
	task.Base
}

func NewDeployIngressNginxTask() task.Task {
	return &DeployIngressNginxTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployIngressNginx",
				Description: "Deploy Ingress-Nginx controller for L7 traffic management",
			},
		},
	}
}

func (t *DeployIngressNginxTask) Name() string {
	return t.Meta.Name
}

func (t *DeployIngressNginxTask) Description() string {
	return t.Meta.Description
}

func (t *DeployIngressNginxTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Gateway == nil || ctx.GetClusterConfig().Spec.Gateway.IngressNginx == nil {
		return false, nil
	}
	if ctx.GetClusterConfig().Spec.Gateway.IngressNginx.Enabled == nil {
		return false, nil
	}
	return *ctx.GetClusterConfig().Spec.Gateway.IngressNginx.Enabled, nil
}

func (t *DeployIngressNginxTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy ingress-nginx")
	}
	executionHost := masterHosts[0]

	generateStep, err := ingressnginx.NewGenerateIngressNginxValuesStepBuilder(runtimeCtx, "GenerateIngressNginxManifests").Build()
	if err != nil {
		return nil, err
	}
	distributeStep, err := ingressnginx.NewDistributeIngressNginxArtifactsStepBuilder(runtimeCtx, "InstallIngressNginx").Build()
	if err != nil {
		return nil, err
	}
	installStep, err := ingressnginx.NewInstallIngressNginxHelmChartStepBuilder(runtimeCtx, "InstallIngressNginx").Build()
	if err != nil {
		return nil, err
	}

	nodeGenerate := &plan.ExecutionNode{Name: "GenerateIngressNginxManifests", Step: generateStep, Hosts: []remotefw.Host{executionHost}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeIngressNginxArtifacts", Step: distributeStep, Hosts: []remotefw.Host{controlNode}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallIngressNginx", Step: installStep, Hosts: []remotefw.Host{executionHost}}

	fragment.AddNode(nodeGenerate)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateIngressNginxManifests", "DistributeIngressNginxArtifacts")
	fragment.AddDependency("DistributeIngressNginxArtifacts", "InstallIngressNginx")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
