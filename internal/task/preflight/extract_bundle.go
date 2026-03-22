package preflight

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	offlinestep "github.com/mensylisir/kubexm/internal/step/offline"
	"github.com/mensylisir/kubexm/internal/task"
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
				Description: "Extract the offline asset bundle on the control node",
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

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHosts := []connector.Host{controlNode}
	if len(executionHosts) == 0 {
		return fragment, nil
	}

	finalBundlePath := t.BundlePath
	if finalBundlePath == "" {
		finalBundlePath = filepath.Join(".", "kubexm-bundle.tar.gz")
	}

	if finalBundlePath == "" {
		return nil, fmt.Errorf("output path for the offline bundle must be specified")
	}

	extractStep, err := offlinestep.NewExtractBundleStepBuilder(runtimeCtx, "ExtractBundle").WithBundlePath(finalBundlePath).Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractBundleOnControlNode", Step: extractStep, Hosts: executionHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
