package harbor

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/harbor"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployHarborTask struct {
	task.Base
}

func NewDeployHarborTask() task.Task {
	return &DeployHarborTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHarbor",
				Description: "Deploy Harbor private image registry",
			},
		},
	}
}

func (t *DeployHarborTask) Name() string {
	return t.Meta.Name
}

func (t *DeployHarborTask) Description() string {
	return t.Meta.Description
}

func (t *DeployHarborTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Registry == nil || ctx.GetClusterConfig().Spec.Registry.LocalDeployment == nil {
		return false, nil
	}
	return ctx.GetClusterConfig().Spec.Registry.LocalDeployment.Type == common.RegistryTypeHarbor, nil
}

func (t *DeployHarborTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, _ := ctx.GetControlNode()
	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	executionHost := registryHosts[0]
	allClusterHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)

	genCerts, err := harbor.NewGenerateHarborCertsStepBuilder(runtimeCtx, "GenerateHarborCerts").Build()
	if err != nil {
		return nil, err
	}
	extractOnControlNode, err := harbor.NewExtractHarborStepBuilder(runtimeCtx, "ExtractHarborOnControlNode").Build()
	if err != nil {
		return nil, err
	}
	genConfig, err := harbor.NewGenerateHarborConfigStepBuilder(runtimeCtx, "GenerateHarborConfig").Build()
	if err != nil {
		return nil, err
	}
	distributeCACerts, err := harbor.NewDistributeHarborCACertStepBuilder(runtimeCtx, "DistributeHarborCA").Build()
	if err != nil {
		return nil, err
	}
	uploadInstaller, err := harbor.NewUploadHarborInstallerStepBuilder(runtimeCtx, "UploadHarborInstaller").Build()
	if err != nil {
		return nil, err
	}
	extractOnRegistryNode, err := harbor.NewExtractHarborInstallerStepBuilder(runtimeCtx, "ExtractHarborOnRegistryNode").Build()
	if err != nil {
		return nil, err
	}
	configureHarbor, err := harbor.NewConfigureHarborStepBuilder(runtimeCtx, "ConfigureHarbor").Build()
	if err != nil {
		return nil, err
	}
	installAndStart, err := harbor.NewInstallAndStartHarborStepBuilder(runtimeCtx, "InstallAndStartHarbor").Build()
	if err != nil {
		return nil, err
	}

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHarborCerts", Step: genCerts, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeHarborCA", Step: distributeCACerts, Hosts: allClusterHosts})
	fragment.AddDependency("GenerateHarborCerts", "DistributeHarborCA")
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractHarborOnControlNode", Step: extractOnControlNode, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHarborConfig", Step: genConfig, Hosts: []remotefw.Host{controlNode}})

	fragment.AddDependency("ExtractHarborOnControlNode", "GenerateHarborConfig")
	fragment.AddDependency("GenerateHarborCerts", "GenerateHarborConfig")
	fragment.AddNode(&plan.ExecutionNode{Name: "UploadHarborInstaller", Step: uploadInstaller, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractHarborOnRegistryNode", Step: extractOnRegistryNode, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureHarbor", Step: configureHarbor, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallAndStartHarbor", Step: installAndStart, Hosts: []remotefw.Host{executionHost}})
	fragment.AddDependency("UploadHarborInstaller", "ExtractHarborOnRegistryNode")
	fragment.AddDependency("ExtractHarborOnRegistryNode", "ConfigureHarbor")
	fragment.AddDependency("ConfigureHarbor", "InstallAndStartHarbor")

	fragment.AddDependency("GenerateHarborConfig", "ConfigureHarbor")
	fragment.AddDependency("DistributeHarborCA", "InstallAndStartHarbor")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
