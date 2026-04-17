package addon

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	helmstep "github.com/mensylisir/kubexm/internal/step/helm"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/types"
)

// CleanAddonTask uninstalls addons from the cluster during deletion.
// It handles both Helm chart-based addons and YAML-based addons.
type CleanAddonTask struct {
	task.Base
	Addon *v1alpha1.Addon
}

func NewCleanAddonTask(addon *v1alpha1.Addon) task.Task {
	return &CleanAddonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        fmt.Sprintf("CleanAddon-%s", addon.Name),
				Description: fmt.Sprintf("Uninstall the '%s' addon from the cluster", addon.Name),
			},
		},
		Addon: addon,
	}
}

func (t *CleanAddonTask) Name() string {
	return t.Meta.Name
}

func (t *CleanAddonTask) Description() string {
	return t.Meta.Description
}

func (t *CleanAddonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Always try to clean addons if they were defined in config
	return t.Addon != nil, nil
}

func (t *CleanAddonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("failed to assert runtime.Context from TaskContext")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		// If we can't get control node, addon cleanup is not possible
		return fragment, nil
	}
	controlNodeList := []remotefw.Host{controlNode}

	var lastExitNodes []plan.NodeID

	for i, source := range t.Addon.Sources {
		var sourceFragment *plan.ExecutionFragment

		switch {
		case source.Yaml != nil:
			sourceFragment, err = t.planYamlSourceCleanup(runtimeCtx, i, &source, controlNodeList)
		case source.Chart != nil:
			sourceFragment, err = t.planHelmSourceCleanup(runtimeCtx, i, &source, controlNodeList)
		default:
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("failed to plan cleanup for addon '%s' source %d: %w", t.Addon.Name, i, err)
		}

		if sourceFragment.IsEmpty() {
			continue
		}

		if err := fragment.MergeFragment(sourceFragment); err != nil {
			return nil, fmt.Errorf("failed to merge fragment for addon '%s' source %d: %w", t.Addon.Name, i, err)
		}

		if len(lastExitNodes) > 0 {
			if err := plan.LinkFragments(fragment, lastExitNodes, sourceFragment.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link fragments for addon '%s' source %d: %w", t.Addon.Name, i, err)
			}
		}
		lastExitNodes = sourceFragment.ExitNodes
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *CleanAddonTask) planYamlSourceCleanup(ctx *runtime.Context, index int, source *v1alpha1.AddonSource, hosts []remotefw.Host) (*plan.ExecutionFragment, error) {
	frag := plan.NewExecutionFragment(fmt.Sprintf("%s-clean-yaml-%d", t.Addon.Name, index))

	// For YAML-based addons, we use kubectl delete if the resources exist
	// Since we don't track what was installed, we attempt delete and ignore errors
	deleteStep, err := NewDeleteAddonYamlStepBuilder(ctx, t.Addon.Name, index).Build()
	if err != nil {
		return nil, err
	}
	if deleteStep == nil {
		return frag, nil
	}
	deleteNode := &plan.ExecutionNode{
		Name:  fmt.Sprintf("Delete-%s-yaml-%d", t.Addon.Name, index),
		Step:  deleteStep,
		Hosts: hosts,
	}
	frag.AddNode(deleteNode)
	frag.CalculateEntryAndExitNodes()
	return frag, nil
}

func (t *CleanAddonTask) planHelmSourceCleanup(ctx *runtime.Context, index int, source *v1alpha1.AddonSource, hosts []remotefw.Host) (*plan.ExecutionFragment, error) {
	frag := plan.NewExecutionFragment(fmt.Sprintf("%s-clean-helm-%d", t.Addon.Name, index))
	helm := source.Chart

	releaseName := helm.Name
	if releaseName == "" {
		releaseName = t.Addon.Name
	}
	namespace := source.Namespace

	// Use helm uninstall to remove the release
	uninstallStep, err := helmstep.NewUninstallChartStepBuilder(ctx, fmt.Sprintf("UninstallChart-%s", releaseName)).
		WithReleaseName(releaseName).
		WithNamespace(namespace).
		Build()
	if err != nil {
		return nil, err
	}

	uninstallNode := &plan.ExecutionNode{
		Name:  fmt.Sprintf("UninstallChart-%s-helm-%d", t.Addon.Name, index),
		Step:  uninstallStep,
		Hosts: hosts,
	}
	frag.AddNode(uninstallNode)
	frag.CalculateEntryAndExitNodes()
	return frag, nil
}

// Ensure task.Task interface is implemented
var _ task.Task = (*CleanAddonTask)(nil)

// DeleteAddonYamlStep deletes Kubernetes resources from a YAML manifest
type DeleteAddonYamlStep struct {
	step.Base
	AddonName string
	Index     int
	Path      string
}

type DeleteAddonYamlStepBuilder struct {
	step.Builder[DeleteAddonYamlStepBuilder, *DeleteAddonYamlStep]
}

func NewDeleteAddonYamlStepBuilder(ctx runtime.ExecutionContext, addonName string, index int) *DeleteAddonYamlStepBuilder {
	s := &DeleteAddonYamlStep{}

	s.Base.Meta.Name = fmt.Sprintf("DeleteAddonYaml-%s-%d", addonName, index)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Delete addon YAML resources", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = true // Ignore errors if resources don't exist
	s.Base.Timeout = 5 * time.Minute

	b := new(DeleteAddonYamlStepBuilder).Init(s)
	return b
}

func (b *DeleteAddonYamlStepBuilder) WithPath(path string) *DeleteAddonYamlStepBuilder {
	b.Step.Path = path
	return b
}

func (s *DeleteAddonYamlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DeleteAddonYamlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Always run if we have a path
	if s.Path == "" {
		return true, nil // Nothing to delete
	}
	return false, nil
}

func (s *DeleteAddonYamlStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	// Delete resources using kubectl delete
	cmd := fmt.Sprintf("kubectl delete -f %s --ignore-not-found", s.Path)
	logger.Infof("Deleting addon YAML resources: %s", cmd)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Warnf("Failed to delete addon YAML (may already be gone): %v", err)
		// Don't fail the step - resources might already be gone
	}

	result.MarkCompleted("addon YAML resources deleted")
	return result, nil
}

func (s *DeleteAddonYamlStep) Rollback(ctx runtime.ExecutionContext) error {
	// Rollback not applicable for delete operations
	return nil
}
