package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	offlinestep "github.com/mensylisir/kubexm/pkg/step/offline"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

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

	compressStep := offlinestep.NewCompressBundleStepBuilder(*runtimeCtx, "CompressBundle").WithOutputPath(finalOutputPath).Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CompressBundle", Step: compressStep, Hosts: []connector.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
