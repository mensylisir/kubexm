package pre

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	commonstep "github.com/mensylisir/kubexm/internal/step/common"
	repositorystep "github.com/mensylisir/kubexm/internal/step/repository"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/pkg/errors"
)

type CreateRepositoryTask struct {
	task.Base
}

func NewCreateRepositoryTask() task.Task {
	return &CreateRepositoryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CreateRepository",
				Description: "Create a local package repository for offline installation",
			},
		},
	}
}

func (t *CreateRepositoryTask) Name() string {
	return t.Meta.Name
}

func (t *CreateRepositoryTask) Description() string {
	return t.Meta.Description
}

func (t *CreateRepositoryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.IsOfflineMode(), nil
}

func (t *CreateRepositoryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	// The repository is usually hosted on the first master node.
	repoHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node to host repository")
	}
	repoHostList := []connector.Host{repoHost}

	// Repository defaults for offline mode
	repoDir := ctx.GetRepositoryDir()
	repoPort := 8080

	// Determine OS family to choose the correct repo creation step
	facts, err := ctx.GetHostFacts(repoHost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get host facts for %s", repoHost.GetName())
	}

	var createRepoMetadataStep step.Step
	switch facts.PackageManager.Type {
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		createRepoMetadataStep, err = repositorystep.NewCreateYumRepoStepBuilder(runtimeCtx, "CreateYumRepoMetadata").
			WithRepoDir(repoDir).
			Build()
		if err != nil {
			return nil, err
		}
	case runner.PackageManagerApt:
		createRepoMetadataStep, err = repositorystep.NewCreateAptRepoStepBuilder(runtimeCtx, "CreateAptRepoMetadata").
			WithRepoDir(repoDir).
			Build()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported package manager '%s' on repository host '%s'", facts.PackageManager.Type, repoHost.GetName())
	}

	ensureRepoDir, err := commonstep.NewMkdirStepBuilder(runtimeCtx, "EnsureRepoDir", repoDir).Build()
	if err != nil {
		return nil, err
	}
	ensureRepoDirNode := &plan.ExecutionNode{Name: "EnsureRepoDir", Step: ensureRepoDir, Hosts: repoHostList}
	ensureRepoDirNodeID, _ := fragment.AddNode(ensureRepoDirNode)

	createRepoMetadataNode := &plan.ExecutionNode{Name: "CreateRepoMetadata", Step: createRepoMetadataStep, Hosts: repoHostList}
	createRepoMetadataNodeID, _ := fragment.AddNode(createRepoMetadataNode)

	// Configure and deploy the NGINX server
	nginxConfigPath := filepath.Join(common.DefaultConfigPath, "repository", "nginx.conf")

	renderNginxConfigStep, err := repositorystep.NewRenderRepoNginxConfigStepBuilder(runtimeCtx, "RenderRepoNginxConfig").
		WithRepoDir(repoDir).
		WithListenPort(repoPort).
		WithConfigPath(nginxConfigPath).
		Build()
	if err != nil {
		return nil, err
	}
	renderNginxConfigNode := &plan.ExecutionNode{Name: "RenderRepoNginxConfig", Step: renderNginxConfigStep, Hosts: repoHostList}
	renderNginxConfigNodeID, _ := fragment.AddNode(renderNginxConfigNode)

	deployNginxPodStep, err := repositorystep.NewDeployRepoNginxPodStepBuilder(runtimeCtx, "DeployRepoNginxPod").
		WithListenPort(repoPort).
		WithConfigPath(nginxConfigPath).
		Build()
	if err != nil {
		return nil, err
	}
	deployNginxPodNode := &plan.ExecutionNode{Name: "DeployRepoNginxPod", Step: deployNginxPodStep, Hosts: repoHostList}
	deployNginxPodNodeID, _ := fragment.AddNode(deployNginxPodNode)

	// Define dependencies
	fragment.AddDependency(ensureRepoDirNodeID, createRepoMetadataNodeID)
	fragment.AddDependency(createRepoMetadataNodeID, renderNginxConfigNodeID)
	fragment.AddDependency(renderNginxConfigNodeID, deployNginxPodNodeID)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
