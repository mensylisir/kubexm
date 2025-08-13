package harbor

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/harbor"
	"github.com/mensylisir/kubexm/pkg/task"
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
	runtimeCtx, _ := ctx.(*runtime.Context)

	controlNode, _ := ctx.GetControlNode()
	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	executionHost := registryHosts[0]
	allClusterHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)

	downloadHarbor := harbor.NewDownloadHarborStepBuilder(*runtimeCtx, "DownloadHarborPackage").Build()
	genCerts := harbor.NewGenerateHarborCertsStepBuilder(*runtimeCtx, "GenerateHarborCerts").Build()
	extractOnControlNode := harbor.NewExtractHarborStepBuilder(*runtimeCtx, "ExtractHarborOnControlNode").Build()
	genConfig := harbor.NewGenerateHarborConfigStepBuilder(*runtimeCtx, "GenerateHarborConfig").Build()
	distributeCACerts := harbor.NewDistributeHarborCACertStepBuilder(*runtimeCtx, "DistributeHarborCA").Build()
	uploadInstaller := harbor.NewUploadHarborInstallerStepBuilder(*runtimeCtx, "UploadHarborInstaller").Build()
	extractOnRegistryNode := harbor.NewExtractHarborInstallerStepBuilder(*runtimeCtx, "ExtractHarborOnRegistryNode").Build()
	configureHarbor := harbor.NewConfigureHarborStepBuilder(*runtimeCtx, "ConfigureHarbor").Build()
	installAndStart := harbor.NewInstallAndStartHarborStepBuilder(*runtimeCtx, "InstallAndStartHarbor").Build()

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadHarborPackage", Step: downloadHarbor, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadHarborPackage", "extractOnControlNode")
		fragment.AddDependency("DownloadHarborPackage", "uploadInstaller")
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHarborCerts", Step: genCerts, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeHarborCA", Step: distributeCACerts, Hosts: allClusterHosts})
	fragment.AddDependency("GenerateHarborCerts", "DistributeHarborCA")
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractHarborOnControlNode", Step: extractOnControlNode, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHarborConfig", Step: genConfig, Hosts: []connector.Host{controlNode}})

	fragment.AddDependency("ExtractHarborOnControlNode", "GenerateHarborConfig")
	fragment.AddDependency("GenerateHarborCerts", "GenerateHarborConfig")
	fragment.AddNode(&plan.ExecutionNode{Name: "UploadHarborInstaller", Step: uploadInstaller, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractHarborOnRegistryNode", Step: extractOnRegistryNode, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureHarbor", Step: configureHarbor, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallAndStartHarbor", Step: installAndStart, Hosts: []connector.Host{executionHost}})
	fragment.AddDependency("UploadHarborInstaller", "ExtractHarborOnRegistryNode")
	fragment.AddDependency("ExtractHarborOnRegistryNode", "ConfigureHarbor")
	fragment.AddDependency("ConfigureHarbor", "InstallAndStartHarbor")

	fragment.AddDependency("GenerateHarborConfig", "ConfigureHarbor")
	fragment.AddDependency("DistributeHarborCA", "InstallAndStartHarbor")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
