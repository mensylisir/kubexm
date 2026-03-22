package flannel

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/flannel"
	"github.com/mensylisir/kubexm/internal/task"
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

	generateStep, err := flannel.NewGenerateFlannelValuesStepBuilder(runtimeCtx, "GenerateFlannelManifest").Build()
	if err != nil {
		return nil, err
	}
	distributeStep, err := flannel.NewDistributeFlannelArtifactsStepBuilder(runtimeCtx, "DistributeFlannelManifest").Build()
	if err != nil {
		return nil, err
	}
	installStep, err := flannel.NewInstallFlannelHelmChartStepBuilder(runtimeCtx, "InstallFlannel").Build()
	if err != nil {
		return nil, err
	}

	nodeGenerate := &plan.ExecutionNode{Name: "GenerateFlannelManifest", Step: generateStep, Hosts: []connector.Host{controlNode}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeFlannelManifest", Step: distributeStep, Hosts: []connector.Host{executionHost}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallFlannel", Step: installStep, Hosts: []connector.Host{executionHost}}

	fragment.AddNode(nodeGenerate)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateFlannelManifest", "DistributeFlannelManifest")
	fragment.AddDependency("DistributeFlannelManifest", "InstallFlannel")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
