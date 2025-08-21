package ingress_nginx

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	ingressnginx "github.com/mensylisir/kubexm/pkg/step/gateway/ingress-nginx"

	"github.com/mensylisir/kubexm/pkg/task"
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
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy ingress-nginx")
	}
	executionHost := masterHosts[0]

	downloadStep := ingressnginx.NewDownloadIngressNginxChartStepBuilder(*runtimeCtx, "DownloadIngressNginx").Build()
	generateStep := ingressnginx.NewGenerateIngressNginxValuesStepBuilder(*runtimeCtx, "GenerateIngressNginxManifests").Build()
	distributeStep := ingressnginx.NewDistributeIngressNginxArtifactsStepBuilder(*runtimeCtx, "InstallIngressNginx").Build()
	installStep := ingressnginx.NewInstallIngressNginxHelmChartStepBuilder(*runtimeCtx, "InstallIngressNginx").Build()

	nodeGenerate := &plan.ExecutionNode{Name: "GenerateIngressNginxManifests", Step: generateStep, Hosts: []connector.Host{executionHost}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeIngressNginxArtifacts", Step: distributeStep, Hosts: []connector.Host{controlNode}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallIngressNginx", Step: installStep, Hosts: []connector.Host{executionHost}}

	fragment.AddNode(nodeGenerate)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateIngressNginxManifests", "DistributeIngressNginxArtifacts")
	fragment.AddDependency("DistributeIngressNginxArtifacts", "InstallIngressNginx")

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		nodeDownload := &plan.ExecutionNode{
			Name:  "DownloadIngressNginx",
			Step:  downloadStep,
			Hosts: []connector.Host{controlNode},
		}
		fragment.AddNode(nodeDownload)
		fragment.AddDependency("DownloadIngressNginx", "GenerateIngressNginxManifests")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
