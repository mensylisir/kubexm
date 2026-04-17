package addon

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	addonstep "github.com/mensylisir/kubexm/internal/step/addon"
	helmstep "github.com/mensylisir/kubexm/internal/step/helm"
	"github.com/mensylisir/kubexm/internal/task"
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
	controlNodeList := []remotefw.Host{controlNode}

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

func (t *InstallAddonTask) planYamlSource(ctx *runtime.Context, index int, source *v1alpha1.AddonSource, hosts []remotefw.Host) (*plan.ExecutionFragment, error) {
	frag := plan.NewExecutionFragment(fmt.Sprintf("%s-yaml-%d", t.Addon.Name, index))

	// 1. Download addon artifacts (YAML files) to local machine
	downloadStep, err := addonstep.NewDownloadAddonArtifactsStepBuilder(ctx, t.Addon.Name).Build()
	if err != nil {
		return nil, err
	}
	if downloadStep != nil {
		downloadNode := &plan.ExecutionNode{
			Name:  fmt.Sprintf("Download-%s-yaml-%d", t.Addon.Name, index),
			Step:  downloadStep,
			Hosts: hosts,
		}
		frag.AddNode(downloadNode)
	}

	// 2. Distribute addon artifacts to remote hosts
	distributeStep, err := addonstep.NewDistributeAddonArtifactsStepBuilder(ctx, t.Addon.Name).Build()
	if err != nil {
		return nil, err
	}
	if distributeStep != nil {
		distributeNode := &plan.ExecutionNode{
			Name:  fmt.Sprintf("Distribute-%s-yaml-%d", t.Addon.Name, index),
			Step:  distributeStep,
			Hosts: hosts,
		}
		frag.AddNode(distributeNode)
	}

	// 3. Apply YAML to cluster
	applyStep, err := addonstep.NewApplyAddonYamlStepBuilder(ctx, t.Addon.Name, index).Build()
	if err != nil {
		return nil, err
	}
	if applyStep != nil {
		applyNode := &plan.ExecutionNode{
			Name:  fmt.Sprintf("Apply-%s-yaml-%d", t.Addon.Name, index),
			Step:  applyStep,
			Hosts: hosts,
		}
		frag.AddNode(applyNode)
	}

	// Set up dependencies
	frag.CalculateEntryAndExitNodes()
	return frag, nil
}

func (t *InstallAddonTask) planHelmSource(ctx *runtime.Context, index int, source *v1alpha1.AddonSource, hosts []remotefw.Host) (*plan.ExecutionFragment, error) {
	frag := plan.NewExecutionFragment(fmt.Sprintf("%s-helm-%d", t.Addon.Name, index))
	helm := source.Chart
	repoName := filepath.Base(helm.Repo) // Simple way to get a repo name

	// Step 1: Download chart artifacts (if local path)
	downloadStep, err := addonstep.NewDownloadAddonArtifactsStepBuilder(ctx, t.Addon.Name).Build()
	if err != nil {
		return nil, err
	}
	var lastExitNode plan.NodeID
	if downloadStep != nil {
		downloadNode := &plan.ExecutionNode{
			Name:  fmt.Sprintf("Download-%s-helm-%d", t.Addon.Name, index),
			Step:  downloadStep,
			Hosts: hosts,
		}
		downloadNodeID, _ := frag.AddNode(downloadNode)
		lastExitNode = downloadNodeID
	}

	// Step 2: Distribute chart artifacts to remote hosts
	distributeStep, err := addonstep.NewDistributeAddonArtifactsStepBuilder(ctx, t.Addon.Name).Build()
	if err != nil {
		return nil, err
	}
	if distributeStep != nil {
		distributeNode := &plan.ExecutionNode{
			Name:  fmt.Sprintf("Distribute-%s-helm-%d", t.Addon.Name, index),
			Step:  distributeStep,
			Hosts: hosts,
		}
		distributeNodeID, _ := frag.AddNode(distributeNode)
		if lastExitNode != "" {
			frag.AddDependency(lastExitNode, distributeNodeID)
		}
		lastExitNode = distributeNodeID
	}

	// Step 3: Add Helm Repo (if specified)
	if helm.Repo != "" {
		addRepoStep, err := helmstep.NewAddRepoStepBuilder(ctx, fmt.Sprintf("AddRepo-%s", repoName)).
			WithRepoName(repoName).
			WithRepoURL(helm.Repo).
			Build()
		if err != nil {
			return nil, err
		}
		addRepoNode := &plan.ExecutionNode{Name: fmt.Sprintf("AddRepo-%s-helm-%d", t.Addon.Name, index), Step: addRepoStep, Hosts: hosts}
		addRepoNodeID, _ := frag.AddNode(addRepoNode)
		if lastExitNode != "" {
			frag.AddDependency(lastExitNode, addRepoNodeID)
		}
		lastExitNode = addRepoNodeID
	}

	// Step 4: Install Chart
	releaseName := helm.Name
	if releaseName == "" {
		releaseName = t.Addon.Name
	}
	namespace := source.Namespace

	chartPath := helm.Name
	if helm.Repo != "" {
		chartPath = fmt.Sprintf("%s/%s", repoName, helm.Name)
	} else if helm.Path != "" {
		chartPath = helm.Path
	}

	installChartStep, err := helmstep.NewInstallChartStepBuilder(ctx, fmt.Sprintf("InstallChart-%s", releaseName)).
		WithChartName(chartPath).
		WithReleaseName(releaseName).
		WithNamespace(namespace).
		WithVersion(helm.Version).
		WithValuesFile(helm.ValuesFile).
		WithExtraArgs(helm.Values).
		Build()
	if err != nil {
		return nil, err
	}
	installChartNode := &plan.ExecutionNode{Name: fmt.Sprintf("InstallChart-%s-helm-%d", t.Addon.Name, index), Step: installChartStep, Hosts: hosts}
	installChartNodeID, _ := frag.AddNode(installChartNode)

	if lastExitNode != "" {
		frag.AddDependency(lastExitNode, installChartNodeID)
	}

	frag.CalculateEntryAndExitNodes()
	return frag, nil
}
