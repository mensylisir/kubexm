package longhorn

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/storage/longhorn"
	"github.com/mensylisir/kubexm/pkg/task"
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
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

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

	downloadLonghorn := longhorn.NewDownloadLonghornChartStepBuilder(*runtimeCtx, "DownloadLonghorn").Build()
	generateManifests := longhorn.NewGenerateLonghornValuesStepBuilder(*runtimeCtx, "GenerateLonghornManifests").Build()
	distributeLonghorn := longhorn.NewDistributeLonghornArtifactsStepBuilder(*runtimeCtx, "DistributeLonghorn").Build()
	installLonghorn := longhorn.NewInstallLonghornHelmChartStepBuilder(*runtimeCtx, "InstallLonghorn").Build()

	nodeGenManifests := &plan.ExecutionNode{Name: "GenerateLonghornManifests", Step: generateManifests, Hosts: []connector.Host{executionHost}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeLonghorn", Step: distributeLonghorn, Hosts: []connector.Host{executionHost}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallLonghorn", Step: installLonghorn, Hosts: []connector.Host{executionHost}}

	fragment.AddNode(nodeGenManifests)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateLonghornManifests", "DistributeLonghorn")
	fragment.AddDependency("DistributeLonghorn", "InstallLonghorn")

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		nodeDownload := &plan.ExecutionNode{Name: "DownloadLonghorn", Step: downloadLonghorn, Hosts: []connector.Host{controlNode}}
		fragment.AddNode(nodeDownload)
		fragment.AddDependency("DownloadLonghorn", "GenerateLonghornManifests")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
