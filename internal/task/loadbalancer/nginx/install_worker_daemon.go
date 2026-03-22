package nginx

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	nginxstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployNginxOnWorkersTask struct {
	task.Base
}

func NewDeployNginxOnWorkersTask() task.Task {
	return &DeployNginxOnWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxOnWorkers",
				Description: "Deploy NGINX as a system service on worker nodes for internal load balancing",
			},
		},
	}
}

func (t *DeployNginxOnWorkersTask) Name() string {
	return t.Meta.Name
}

func (t *DeployNginxOnWorkersTask) Description() string {
	return t.Meta.Description
}

func (t *DeployNginxOnWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint == nil || cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil {
		return false, nil
	}
	ha := cfg.Spec.ControlPlaneEndpoint.HighAvailability
	if ha.Enabled == nil || !*ha.Enabled {
		return false, nil
	}
	if ha.Internal == nil || ha.Internal.Enabled == nil || !*ha.Internal.Enabled {
		return false, nil
	}
	if ha.Internal.Type != string(common.InternalLBTypeNginx) {
		return false, nil
	}
	if cfg.Spec.Kubernetes == nil || cfg.Spec.Kubernetes.Type != string(common.KubernetesDeploymentTypeKubexm) {
		return false, nil
	}
	if len(ctx.GetHostsByRole(common.RoleMaster)) <= 1 {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleWorker)) > 0, nil
}

func (t *DeployNginxOnWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	generateConfig, err := nginxstep.NewGenerateNginxConfigStepBuilder(runtimeCtx, "GenerateNginxWorkerConfig").Build()
	if err != nil {
		return nil, err
	}
	enableService, err := nginxstep.NewEnableNginxStepBuilder(runtimeCtx, "EnableNginxWorkerService").Build()
	if err != nil {
		return nil, err
	}
	restartService, err := nginxstep.NewRestartNginxStepBuilder(runtimeCtx, "RestartNginxWorkerService").Build()
	if err != nil {
		return nil, err
	}
	installPackage, err := nginxstep.NewInstallNginxStepBuilder(runtimeCtx, "InstallNginxWorkerPackage").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxWorkerConfig", Step: generateConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableNginxWorkerService", Step: enableService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartNginxWorkerService", Step: restartService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNginxWorkerPackage", Step: installPackage, Hosts: workerHosts})

	fragment.AddDependency("InstallNginxWorkerPackage", "GenerateNginxWorkerConfig")
	fragment.AddDependency("GenerateNginxWorkerConfig", "EnableNginxWorkerService")
	fragment.AddDependency("EnableNginxWorkerService", "RestartNginxWorkerService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployNginxOnWorkersTask)(nil)
