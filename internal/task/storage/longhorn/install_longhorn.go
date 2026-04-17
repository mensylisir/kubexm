package longhorn

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/storage/longhorn"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployLonghornTask struct {
	task.Base
}

func NewDeployLonghornTask() task.Task {
	return &DeployLonghornTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployLonghorn",
				Description: "Deploy Longhorn distributed block storage system",
			},
		},
	}
}

func (t *DeployLonghornTask) Name() string {
	return t.Meta.Name
}

func (t *DeployLonghornTask) Description() string {
	return t.Meta.Description
}

func (t *DeployLonghornTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Storage == nil {
		return false, nil
	}
	return *ctx.GetClusterConfig().Spec.Storage.Longhorn.Enabled, nil
}

func (t *DeployLonghornTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy longhorn")
	}
	executionHost := masterHosts[0]
	//allHosts := append(masterHosts, ctx.GetHostsByRole(common.RoleWorker)...)

	generateManifests, err := longhorn.NewGenerateLonghornValuesStepBuilder(runtimeCtx, "GenerateLonghornManifests").Build()
	if err != nil {
		return nil, err
	}
	distributeLonghorn, err := longhorn.NewDistributeLonghornArtifactsStepBuilder(runtimeCtx, "DistributeLonghorn").Build()
	if err != nil {
		return nil, err
	}
	installLonghorn, err := longhorn.NewInstallLonghornHelmChartStepBuilder(runtimeCtx, "InstallLonghorn").Build()
	if err != nil {
		return nil, err
	}

	nodeGenManifests := &plan.ExecutionNode{Name: "GenerateLonghornManifests", Step: generateManifests, Hosts: []remotefw.Host{executionHost}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeLonghorn", Step: distributeLonghorn, Hosts: []remotefw.Host{executionHost}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallLonghorn", Step: installLonghorn, Hosts: []remotefw.Host{executionHost}}

	fragment.AddNode(nodeGenManifests)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateLonghornManifests", "DistributeLonghorn")
	fragment.AddDependency("DistributeLonghorn", "InstallLonghorn")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
