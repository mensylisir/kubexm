package addon

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	addonstep "github.com/mensylisir/kubexm/pkg/step/addon"
	helmstep "github.com/mensylisir/kubexm/pkg/step/helm"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/pkg/errors"
)

type InstallAddonTask struct {
	task.Base
	Addon *v1alpha1.Addon
}

func NewInstallAddonTask(addon *v1alpha1.Addon) task.Task {
	return &InstallAddonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        fmt.Sprintf("InstallAddon-%s", addon.Name),
				Description: fmt.Sprintf("Install the '%s' addon to the cluster", addon.Name),
			},
		},
		Addon: addon,
	}
}

func (t *InstallAddonTask) Name() string {
	return t.Meta.Name
}

func (t *InstallAddonTask) Description() string {
	return t.Meta.Description
}

func (t *InstallAddonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if t.Addon == nil || (t.Addon.Enabled != nil && !*t.Addon.Enabled) {
		return false, nil
	}
	return true, nil
}

func (t *InstallAddonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, errors.New("failed to assert runtime.Context from TaskContext")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get control node for addon installation")
	}
	controlNodeList := []connector.Host{controlNode}

	var lastSourceExitNodes []plan.NodeID

	for i, source := range t.Addon.Sources {
		var sourceFragment *plan.ExecutionFragment
		var err error

		switch {
		case source.Yaml != nil:
			sourceFragment, err = t.planYamlSource(runtimeCtx, i, &source, controlNodeList)
		case source.Chart != nil:
			sourceFragment, err = t.planHelmSource(runtimeCtx, i, &source, controlNodeList)
		default:
			return nil, fmt.Errorf("addon '%s' source %d has no supported type (yaml or chart)", t.Addon.Name, i)
		}

		if err != nil {
			return nil, errors.Wrapf(err, "failed to plan addon '%s' source %d", t.Addon.Name, i)
		}

		if err := fragment.MergeFragment(sourceFragment); err != nil {
			return nil, errors.Wrapf(err, "failed to merge fragment for addon '%s' source %d", t.Addon.Name, i)
		}

		if i > 0 && len(lastSourceExitNodes) > 0 {
			plan.LinkFragments(fragment, lastSourceExitNodes, sourceFragment.EntryNodes)
		}
		lastSourceExitNodes = sourceFragment.ExitNodes
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *InstallAddonTask) planYamlSource(ctx *runtime.Context, index int, source *v1alpha1.AddonSource, hosts []connector.Host) (*plan.ExecutionFragment, error) {
	frag := plan.NewExecutionFragment(fmt.Sprintf("%s-yaml-%d", t.Addon.Name, index))

	// A full implementation would have download/distribute steps.
	// For now, we just plan the apply step.
	applyStep := addonstep.NewApplyAddonYamlStepBuilder(*ctx, t.Addon.Name, index).Build()
	if applyStep == nil {
		// This happens if the builder determines there's nothing to do.
		return frag, nil
	}
	applyNode := &plan.ExecutionNode{
		Name:  fmt.Sprintf("Apply-%s-yaml-%d", t.Addon.Name, index),
		Step:  applyStep,
		Hosts: hosts,
	}
	frag.AddNode(applyNode)
	frag.CalculateEntryAndExitNodes()
	return frag, nil
}

func (t *InstallAddonTask) planHelmSource(ctx *runtime.Context, index int, source *v1alpha1.AddonSource, hosts []connector.Host) (*plan.ExecutionFragment, error) {
	frag := plan.NewExecutionFragment(fmt.Sprintf("%s-helm-%d", t.Addon.Name, index))
	helm := source.Chart
	repoName := filepath.Base(helm.Repo) // Simple way to get a repo name

	// Step 1: Add Helm Repo (if specified)
	var lastExitNode plan.NodeID
	if helm.Repo != "" {
		addRepoStep := helmstep.NewAddRepoStepBuilder(*ctx, fmt.Sprintf("AddRepo-%s", repoName)).
			WithRepoName(repoName).
			WithRepoURL(helm.Repo).
			Build()
		addRepoNode := &plan.ExecutionNode{Name: fmt.Sprintf("AddRepo-%s-helm-%d", t.Addon.Name, index), Step: addRepoStep, Hosts: hosts}
		nodeID, _ := frag.AddNode(addRepoNode)
		lastExitNode = nodeID
	}

	// Step 2: Install Chart
	releaseName := helm.Name
	if releaseName == "" {
		releaseName = t.Addon.Name
	}
	namespace := source.Namespace

	chartPath := helm.Name
	if helm.Repo != "" {
		chartPath = fmt.Sprintf("%s/%s", repoName, helm.Name)
	} else if helm.Path != "" {
		// A full implementation would distribute this path to the node.
		// For now, assume it's pre-existing.
		chartPath = helm.Path
	}

	installChartStep := helmstep.NewInstallChartStepBuilder(*ctx, fmt.Sprintf("InstallChart-%s", releaseName)).
		WithChartName(chartPath).
		WithReleaseName(releaseName).
		WithNamespace(namespace).
		WithVersion(helm.Version).
		WithValuesFile(helm.ValuesFile).
		WithExtraArgs(helm.Values).
		Build()
	installChartNode := &plan.ExecutionNode{Name: fmt.Sprintf("InstallChart-%s-helm-%d", t.Addon.Name, index), Step: installChartStep, Hosts: hosts}
	installChartNodeID, _ := frag.AddNode(installChartNode)

	if lastExitNode != "" {
		frag.AddDependency(lastExitNode, installChartNodeID)
	}

	frag.CalculateEntryAndExitNodes()
	return frag, nil
}
