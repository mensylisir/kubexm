package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	imagesstep "github.com/mensylisir/kubexm/pkg/step/images"
	"github.com/mensylisir/kubexm/pkg/task"
)

type PushImagesToRegistryTask struct {
	task.Base
}

func NewPushImagesToRegistryTask() task.Task {
	return &PushImagesToRegistryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PushImagesToRegistry",
				Description: "Push all saved container images to the local or specified private registry",
			},
		},
	}
}

func (t *PushImagesToRegistryTask) Name() string {
	return t.Meta.Name
}

func (t *PushImagesToRegistryTask) Description() string {
	return t.Meta.Description
}

func (t *PushImagesToRegistryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if ctx.IsOfflineMode() {
		return true, nil
	}
	if cfg.Spec.Registry != nil && cfg.Spec.Registry.MirroringAndRewriting != nil && cfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry != "" {
		return true, nil
	}
	return false, nil
}

func (t *PushImagesToRegistryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node to push images: %w", err)
	}

	pushImages := imagesstep.NewPushImagesStepBuilder(*runtimeCtx, "PushAllSavedImages").Build()
	pushManifests := imagesstep.NewPushManifestListStepBuilder(*runtimeCtx, "PushMultiArchManifests").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "PushAllSavedImages", Step: pushImages, Hosts: []connector.Host{controlNode}})

	fragment.AddNode(&plan.ExecutionNode{Name: "PushMultiArchManifests", Step: pushManifests, Hosts: []connector.Host{controlNode}})

	fragment.AddDependency("PushAllSavedImages", "PushMultiArchManifests")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
