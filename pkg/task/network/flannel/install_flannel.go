package flannel

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/flannel"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployFlannelTask struct {
	task.Base
}

func NewDeployFlannelTask() task.Task {
	return &DeployFlannelTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployFlannel",
				Description: "Deploy Flannel CNI network addon",
			},
		},
	}
}

func (t *DeployFlannelTask) Name() string        { return t.Meta.Name }
func (t *DeployFlannelTask) Description() string { return t.Meta.Description }

func (t *DeployFlannelTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeFlannel), nil
}

func (t *DeployFlannelTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, _ := ctx.GetControlNode()
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy flannel")
	}
	executionHost := masterHosts[0]

	downloadStep := flannel.NewDownloadFlannelChartStepBuilder(*runtimeCtx, "DownloadFlannelManifest").Build()
	generateStep := flannel.NewGenerateFlannelValuesStepBuilder(*runtimeCtx, "GenerateFlannelManifest").Build()
	distributeStep := flannel.NewDistributeFlannelArtifactsStepBuilder(*runtimeCtx, "DistributeFlannelManifest").Build()
	installStep := flannel.NewInstallFlannelHelmChartStepBuilder(*runtimeCtx, "InstallFlannel").Build()

	nodeGenerate := &plan.ExecutionNode{Name: "GenerateFlannelManifest", Step: generateStep, Hosts: []connector.Host{controlNode}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeFlannelManifest", Step: distributeStep, Hosts: []connector.Host{executionHost}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallFlannel", Step: installStep, Hosts: []connector.Host{executionHost}}

	fragment.AddNode(nodeGenerate)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateFlannelManifest", "DistributeFlannelManifest")
	fragment.AddDependency("DistributeFlannelManifest", "InstallFlannel")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Adding download step for network plugins.")
		nodeDownload := &plan.ExecutionNode{Name: "DownloadFlannelManifest", Step: downloadStep, Hosts: []connector.Host{controlNode}}
		fragment.AddNode(nodeDownload)
		fragment.AddDependency("DownloadFlannelManifest", "GenerateFlannelManifest")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download step for CNI plugins.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
