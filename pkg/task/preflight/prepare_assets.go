package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	addonstep "github.com/mensylisir/kubexm/pkg/step/addon"
	binarystep "github.com/mensylisir/kubexm/pkg/step/binary"
	helmstep "github.com/mensylisir/kubexm/pkg/step/helm"
	imagesstep "github.com/mensylisir/kubexm/pkg/step/images"
	"github.com/mensylisir/kubexm/pkg/task"
)

type PrepareAssetsTask struct {
	task.Base
}

func NewPrepareAssetsTask() task.Task {
	return &PrepareAssetsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PrepareAssets",
				Description: "Download all required binaries, Helm charts, and container images to the control node",
			},
		},
	}
}

func (t *PrepareAssetsTask) Name() string {
	return t.Meta.Name
}

func (t *PrepareAssetsTask) Description() string {
	return t.Meta.Description
}

func (t *PrepareAssetsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return !ctx.IsOfflineMode(), nil
}

func (t *PrepareAssetsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHosts := []connector.Host{controlNode}

	downloadBinaries := binarystep.NewDownloadBinariesStepBuilder(*runtimeCtx, "DownloadBinaries").Build()
	downloadCharts := helmstep.NewDownloadHelmCharts2StepBuilder(*runtimeCtx, "DownloadHelmCharts").Build()
	saveImages := imagesstep.NewSaveImagesStepBuilder(*runtimeCtx, "SaveContainerImages").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "DownloadBinaries", Step: downloadBinaries, Hosts: executionHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DownloadHelmCharts", Step: downloadCharts, Hosts: executionHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "SaveContainerImages", Step: saveImages, Hosts: executionHosts})

	for _, addon := range ctx.GetClusterConfig().Spec.Addons {
		hasRemoteSource := false
		if addon.Enabled != nil && *addon.Enabled {
			for _, source := range addon.Sources {
				if source.Chart != nil {
					hasRemoteSource = true
					break
				}
				if source.Yaml != nil {
					hasRemoteSource = true
					break
				}
			}
		}

		if hasRemoteSource {
			addonName := addon.Name
			downloadAddonStep := addonstep.NewDownloadAddonArtifactsStepBuilder(*runtimeCtx, addonName).Build()
			if downloadAddonStep != nil {
				nodeName := fmt.Sprintf("DownloadAddonArtifacts-%s", addonName)
				fragment.AddNode(&plan.ExecutionNode{Name: nodeName, Step: downloadAddonStep, Hosts: executionHosts})
			}
		}
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
