package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	addonstep "github.com/mensylisir/kubexm/internal/step/addon"
	binarystep "github.com/mensylisir/kubexm/internal/step/binary"
	helmstep "github.com/mensylisir/kubexm/internal/step/helm"
	imagesstep "github.com/mensylisir/kubexm/internal/step/images"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHosts := []remotefw.Host{controlNode}

	downloadBinaries, err := binarystep.NewDownloadBinariesStepBuilder(runtimeCtx, "DownloadBinaries").Build()
	if err != nil {
		return nil, err
	}
	downloadCharts, err := helmstep.NewDownloadHelmCharts2StepBuilder(runtimeCtx, "DownloadHelmCharts").Build()
	if err != nil {
		return nil, err
	}
	saveImages, err := imagesstep.NewSaveImagesStepBuilder(runtimeCtx, "SaveContainerImages").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "DownloadBinaries", Step: downloadBinaries, Hosts: executionHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DownloadHelmCharts", Step: downloadCharts, Hosts: executionHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "SaveContainerImages", Step: saveImages, Hosts: executionHosts})

	fragment.AddDependency("DownloadBinaries", "DownloadHelmCharts")

	for _, addon := range ctx.GetClusterConfig().Spec.Addons {
		if addon.Name == "" {
			continue
		}
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
			downloadAddonStep, err := addonstep.NewDownloadAddonArtifactsStepBuilder(runtimeCtx, addonName).Build()
			if err != nil {
				return nil, err
			}
			if downloadAddonStep != nil {
				nodeName := fmt.Sprintf("DownloadAddonArtifacts-%s", addonName)
				fragment.AddNode(&plan.ExecutionNode{Name: nodeName, Step: downloadAddonStep, Hosts: executionHosts})
			}
		}
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
