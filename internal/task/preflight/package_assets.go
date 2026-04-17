package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	offlinestep "github.com/mensylisir/kubexm/internal/step/offline"
	"github.com/mensylisir/kubexm/internal/task"
	"path/filepath"
)

type PackageAssetsTask struct {
	task.Base
	OutputPath string
}

func NewPackageAssetsTask(outputPath string) task.Task {
	return &PackageAssetsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PackageAssets",
				Description: "Package all downloaded assets into a single offline bundle (.tar.gz)",
			},
		},
		OutputPath: outputPath,
	}
}

func (t *PackageAssetsTask) Name() string {
	return t.Meta.Name
}

func (t *PackageAssetsTask) Description() string {
	return t.Meta.Description
}

func (t *PackageAssetsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *PackageAssetsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	finalOutputPath := t.OutputPath
	if finalOutputPath == "" {
		finalOutputPath = filepath.Join(".", "kubexm-bundle.tar.gz")
	}

	if finalOutputPath == "" {
		return nil, fmt.Errorf("output path for the offline bundle must be specified")
	}

	compressStep, err := offlinestep.NewCompressBundleStepBuilder(runtimeCtx, "CompressBundle").WithOutputPath(finalOutputPath).Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "CompressBundle", Step: compressStep, Hosts: []remotefw.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
