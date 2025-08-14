package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	offlinestep "github.com/mensylisir/kubexm/pkg/step/offline"
	"github.com/mensylisir/kubexm/pkg/task"
	"path/filepath"
)

type ExtractBundleTask struct {
	task.Base
	BundlePath string
}

func NewExtractBundleTask(bundlePath string) task.Task {
	return &ExtractBundleTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ExtractBundle",
				Description: "Extract the offline asset bundle on target nodes",
			},
		},
		BundlePath: bundlePath,
	}
}

func (t *ExtractBundleTask) Name() string {
	return t.Meta.Name
}

func (t *ExtractBundleTask) Description() string {
	return t.Meta.Description
}

func (t *ExtractBundleTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.IsOfflineMode(), nil
}

func (t *ExtractBundleTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	finalBundlePath := t.BundlePath
	if finalBundlePath == "" {
		finalBundlePath = filepath.Join(".", "kubexm-bundle.tar.gz")
	}

	if finalBundlePath == "" {
		return nil, fmt.Errorf("output path for the offline bundle must be specified")
	}

	extractStep := offlinestep.NewExtractBundleStepBuilder(*runtimeCtx, "ExtractBundle").WithBundlePath(finalBundlePath).Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractBundleOnAllNodes", Step: extractStep, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
