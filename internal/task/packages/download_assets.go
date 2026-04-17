package packages

import (
	"github.com/mensylisir/kubexm/internal/asset"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/binary"
	"github.com/mensylisir/kubexm/internal/task"
)

// DownloadAssetsTask downloads all required assets (binaries, images, helm charts) for offline installation.
type DownloadAssetsTask struct {
	task.Base
}

func NewDownloadAssetsTask() task.Task {
	return &DownloadAssetsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DownloadAssets",
				Description: "Download all required binaries, images, and helm charts for offline installation",
			},
		},
	}
}

func (t *DownloadAssetsTask) Name() string        { return t.Meta.Name }
func (t *DownloadAssetsTask) Description() string { return t.Meta.Description }

func (t *DownloadAssetsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *DownloadAssetsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	localHost := remotefw.NewLocalHost()

	// Download binaries
	downloadBinariesStep, err := binary.NewDownloadBinariesStepBuilder(execCtx, "DownloadBinaries").Build()
	if err != nil {
		return nil, err
	}
	node := &plan.ExecutionNode{
		Name:  "DownloadBinaries",
		Step:  downloadBinariesStep,
		Hosts: []remotefw.Host{localHost},
	}
	fragment.AddNode(node)

	// Optionally add image download and helm download nodes
	// These can be added once SaveImagesStep and DownloadHelmChartsStep are integrated with AssetManager

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// AssetManagerTask is a task that uses the centralized AssetManager for unified asset operations.
type AssetManagerTask struct {
	task.Base
}

func NewAssetManagerTask() task.Task {
	return &AssetManagerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "AssetManager",
				Description: "Centralized asset management: query, download, verify, and cache all cluster assets",
			},
		},
	}
}

func (t *AssetManagerTask) Name() string        { return t.Meta.Name }
func (t *AssetManagerTask) Description() string { return t.Meta.Description }

func (t *AssetManagerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *AssetManagerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	localHost := remotefw.NewLocalHost()

	// The AssetManagerStep downloads all assets (binaries, images, helm) in one unified step
	assetStep, err := asset.NewAssetManagerStepBuilder(execCtx, "AssetManager").Build()
	if err != nil {
		return nil, err
	}

	node := &plan.ExecutionNode{
		Name:  "AssetManager",
		Step:  assetStep,
		Hosts: []remotefw.Host{localHost},
	}
	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
