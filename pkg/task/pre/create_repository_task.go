package pre

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	repositorystep "github.com/mensylisir/kubexm/pkg/step/repository"
	"github.com/mensylisir/kubexm/pkg/task"
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

	// Get parameters from config
	// Assuming these fields exist in the spec for offline mode
	repoDir := "/var/www/html/kubexm/packages" // Example path
	repoPort := 8080                          // Example port
	if ctx.GetClusterConfig().Spec.Offline != nil {
		if ctx.GetClusterConfig().Spec.Offline.RepoDir != "" {
			repoDir = ctx.GetClusterConfig().Spec.Offline.RepoDir
		}
		if ctx.GetClusterConfig().Spec.Offline.RepoPort != 0 {
			repoPort = ctx.GetClusterConfig().Spec.Offline.RepoPort
		}
	}

	// Determine OS family to choose the correct repo creation step
	facts, err := ctx.GetHostFacts(repoHost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get host facts for %s", repoHost.GetName())
	}

	var createRepoMetadataStep step.Step
	switch facts.PackageManager.Type {
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		createRepoMetadataStep = repositorystep.NewCreateYumRepoStepBuilder(*runtimeCtx, "CreateYumRepoMetadata").
			WithRepoDir(repoDir).
			Build()
	case runner.PackageManagerApt:
		createRepoMetadataStep = repositorystep.NewCreateAptRepoStepBuilder(*runtimeCtx, "CreateAptRepoMetadata").
			WithRepoDir(repoDir).
			Build()
	default:
		return nil, fmt.Errorf("unsupported package manager '%s' on repository host '%s'", facts.PackageManager.Type, repoHost.GetName())
	}

	createRepoMetadataNode := &plan.ExecutionNode{Name: "CreateRepoMetadata", Step: createRepoMetadataStep, Hosts: repoHostList}
	createRepoMetadataNodeID, _ := fragment.AddNode(createRepoMetadataNode)

	// Configure and deploy the NGINX server
	nginxConfigPath := filepath.Join(common.KubeXMETCDir, "repository", "nginx.conf")

	renderNginxConfigStep := repositorystep.NewRenderRepoNginxConfigStepBuilder(*runtimeCtx, "RenderRepoNginxConfig").
		WithRepoDir(repoDir).
		WithListenPort(repoPort).
		WithConfigPath(nginxConfigPath).
		Build()
	renderNginxConfigNode := &plan.ExecutionNode{Name: "RenderRepoNginxConfig", Step: renderNginxConfigStep, Hosts: repoHostList}
	renderNginxConfigNodeID, _ := fragment.AddNode(renderNginxConfigNode)

	deployNginxPodStep := repositorystep.NewDeployRepoNginxPodStepBuilder(*runtimeCtx, "DeployRepoNginxPod").
		WithListenPort(repoPort).
		WithConfigPath(nginxConfigPath).
		Build()
	deployNginxPodNode := &plan.ExecutionNode{Name: "DeployRepoNginxPod", Step: deployNginxPodStep, Hosts: repoHostList}
	deployNginxPodNodeID, _ := fragment.AddNode(deployNginxPodNode)

	// Define dependencies
	fragment.AddDependency(createRepoMetadataNodeID, renderNginxConfigNodeID)
	fragment.AddDependency(renderNginxConfigNodeID, deployNginxPodNodeID)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
