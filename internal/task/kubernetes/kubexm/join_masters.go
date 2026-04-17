package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	certsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/certs"
	apiserverstep "github.com/mensylisir/kubexm/internal/step/kubernetes/apiserver"
	cmstep "github.com/mensylisir/kubexm/internal/step/kubernetes/controller-manager"
	kubeletstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	proxystep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	schedulerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/scheduler"
	"github.com/mensylisir/kubexm/internal/task"
)

// JoinMastersTask handles joining additional master nodes in kubexm binary deployment mode.
// It distributes all control plane binaries, certificates, configs, and starts all services.
type JoinMastersTask struct {
	task.Base
}

// NewJoinMastersTask creates a new JoinMastersTask for kubexm mode.
func NewJoinMastersTask() task.Task {
	return &JoinMastersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "JoinMasters",
				Description: "Join additional master nodes to the Kubernetes cluster (kubexm binary mode)",
			},
		},
	}
}

func (t *JoinMastersTask) Name() string        { return t.Meta.Name }
func (t *JoinMastersTask) Description() string { return t.Meta.Description }

func (t *JoinMastersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *JoinMastersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	joinMasterHosts := masterHosts[1:]

	if len(joinMasterHosts) == 0 {
		ctx.GetLogger().Info("No additional master nodes to join, skipping task.")
		return fragment, nil
	}

	// ========== Phase 1: Distribute certificates and kubeconfigs ==========
	distributeCerts, err := certsstep.NewDistributeKubeCertsStepBuilder(runtimeCtx, "DistributeAllCertsToJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build distribute certs step: %w", err)
	}
	distributeKubeconfigs, err := certsstep.NewDistributeKubeconfigsStepBuilder(runtimeCtx, "DistributeKubeconfigsToJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build distribute kubeconfigs step: %w", err)
	}
	distributeKubeletCerts, err := certsstep.NewDistributeKubeCACertsStepBuilder(runtimeCtx, "DistributeKubeletCertsToJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build distribute kubelet certs step: %w", err)
	}

	// ========== Phase 2: Install control plane binaries ==========
	installAPIServer, err := apiserverstep.NewInstallKubeApiServerStepBuilder(runtimeCtx, "InstallKubeAPIServerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install apiserver step: %w", err)
	}
	installCM, err := cmstep.NewInstallKubeControllerManagerStepBuilder(runtimeCtx, "InstallKubeControllerManagerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install controller-manager step: %w", err)
	}
	installScheduler, err := schedulerstep.NewInstallKubeSchedulerStepBuilder(runtimeCtx, "InstallKubeSchedulerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install scheduler step: %w", err)
	}
	installKubelet, err := kubeletstep.NewInstallKubeletStepBuilder(runtimeCtx, "InstallKubeletOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install kubelet step: %w", err)
	}
	installProxy, err := proxystep.NewInstallKubeProxyStepBuilder(runtimeCtx, "InstallKubeProxyOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install kube-proxy step: %w", err)
	}

	// ========== Phase 3: Configure control plane components ==========
	configureAPIServer, err := apiserverstep.NewConfigureKubeAPIServerStepBuilder(runtimeCtx, "ConfigureKubeAPIServerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build configure apiserver step: %w", err)
	}
	installAPIServerService, err := apiserverstep.NewInstallKubeAPIServerServiceStepBuilder(runtimeCtx, "InstallKubeAPIServerServiceOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install apiserver service step: %w", err)
	}
	configureCM, err := cmstep.NewConfigureKubeControllerManagerStepBuilder(runtimeCtx, "ConfigureKubeControllerManagerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build configure controller-manager step: %w", err)
	}
	installCMService, err := cmstep.NewInstallKubeControllerManagerServiceStepBuilder(runtimeCtx, "InstallKubeControllerManagerServiceOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install controller-manager service step: %w", err)
	}
	configureScheduler, err := schedulerstep.NewConfigureKubeSchedulerStepBuilder(runtimeCtx, "ConfigureKubeSchedulerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build configure scheduler step: %w", err)
	}
	installSchedulerService, err := schedulerstep.NewInstallKubeSchedulerServiceStepBuilder(runtimeCtx, "InstallKubeSchedulerServiceOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install scheduler service step: %w", err)
	}

	// ========== Phase 4: Configure kubelet and kube-proxy ==========
	createKubeletConfig, err := kubeletstep.NewCreateKubeletConfigYAMLStepBuilder(runtimeCtx, "CreateKubeletConfigForJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build create kubelet config step: %w", err)
	}
	installKubeletService, err := kubeletstep.NewInstallKubeletServiceStepBuilder(runtimeCtx, "InstallKubeletServiceOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install kubelet service step: %w", err)
	}
	installKubeletDropIn, err := kubeletstep.NewInstallKubeletDropInStepBuilder(runtimeCtx, "InstallKubeletDropInOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install kubelet drop-in step: %w", err)
	}
	createProxyConfig, err := proxystep.NewCreateKubeProxyConfigYAMLStepBuilder(runtimeCtx, "CreateKubeProxyConfigForJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build create kube-proxy config step: %w", err)
	}
	installProxyService, err := proxystep.NewInstallKubeProxyServiceStepBuilder(runtimeCtx, "InstallKubeProxyServiceOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build install kube-proxy service step: %w", err)
	}

	// ========== Phase 5: Enable and start all services ==========
	enableAPIServer, err := apiserverstep.NewEnableKubeAPIServerStepBuilder(runtimeCtx, "EnableKubeAPIServerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build enable apiserver step: %w", err)
	}
	restartAPIServer, err := apiserverstep.NewRestartKubeApiServerStepBuilder(runtimeCtx, "RestartKubeAPIServerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build restart apiserver step: %w", err)
	}
	checkAPIServerHealth, err := apiserverstep.NewCheckAPIServerHealthStepBuilder(runtimeCtx, "CheckKubeAPIServerHealthOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build check apiserver health step: %w", err)
	}
	enableCM, err := cmstep.NewEnableKubeControllerManagerStepBuilder(runtimeCtx, "EnableKubeControllerManagerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build enable controller-manager step: %w", err)
	}
	startCM, err := cmstep.NewStartKubeControllerManagerStepBuilder(runtimeCtx, "StartKubeControllerManagerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build start controller-manager step: %w", err)
	}
	enableScheduler, err := schedulerstep.NewEnableKubeSchedulerStepBuilder(runtimeCtx, "EnableKubeSchedulerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build enable scheduler step: %w", err)
	}
	startScheduler, err := schedulerstep.NewStartKubeSchedulerStepBuilder(runtimeCtx, "StartKubeSchedulerOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build start scheduler step: %w", err)
	}
	enableKubelet, err := kubeletstep.NewEnableKubeletStepBuilder(runtimeCtx, "EnableKubeletOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build enable kubelet step: %w", err)
	}
	startKubelet, err := kubeletstep.NewStartKubeletStepBuilder(runtimeCtx, "StartKubeletOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build start kubelet step: %w", err)
	}
	enableProxy, err := proxystep.NewEnableKubeProxyStepBuilder(runtimeCtx, "EnableKubeProxyOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build enable kube-proxy step: %w", err)
	}
	startProxy, err := proxystep.NewStartKubeProxyStepBuilder(runtimeCtx, "StartKubeProxyOnJoinMasters").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build start kube-proxy step: %w", err)
	}

	// ========== Add all nodes to fragment ==========
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeAllCertsToJoinMasters", Step: distributeCerts, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeKubeconfigsToJoinMasters", Step: distributeKubeconfigs, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeKubeletCertsToJoinMasters", Step: distributeKubeletCerts, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeAPIServerOnJoinMasters", Step: installAPIServer, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeControllerManagerOnJoinMasters", Step: installCM, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeSchedulerOnJoinMasters", Step: installScheduler, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletOnJoinMasters", Step: installKubelet, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeProxyOnJoinMasters", Step: installProxy, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeAPIServerOnJoinMasters", Step: configureAPIServer, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeAPIServerServiceOnJoinMasters", Step: installAPIServerService, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeControllerManagerOnJoinMasters", Step: configureCM, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeControllerManagerServiceOnJoinMasters", Step: installCMService, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeSchedulerOnJoinMasters", Step: configureScheduler, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeSchedulerServiceOnJoinMasters", Step: installSchedulerService, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeletConfigForJoinMasters", Step: createKubeletConfig, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletServiceOnJoinMasters", Step: installKubeletService, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletDropInOnJoinMasters", Step: installKubeletDropIn, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeProxyConfigForJoinMasters", Step: createProxyConfig, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeProxyServiceOnJoinMasters", Step: installProxyService, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeAPIServerOnJoinMasters", Step: enableAPIServer, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartKubeAPIServerOnJoinMasters", Step: restartAPIServer, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckKubeAPIServerHealthOnJoinMasters", Step: checkAPIServerHealth, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeControllerManagerOnJoinMasters", Step: enableCM, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeControllerManagerOnJoinMasters", Step: startCM, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeSchedulerOnJoinMasters", Step: enableScheduler, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeSchedulerOnJoinMasters", Step: startScheduler, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeletOnJoinMasters", Step: enableKubelet, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeletOnJoinMasters", Step: startKubelet, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeProxyOnJoinMasters", Step: enableProxy, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeProxyOnJoinMasters", Step: startProxy, Hosts: joinMasterHosts})

	// ========== Set up dependencies ==========
	// Certs and kubeconfigs must be distributed before configuring components
	fragment.AddDependency("DistributeAllCertsToJoinMasters", "ConfigureKubeAPIServerOnJoinMasters")
	fragment.AddDependency("DistributeKubeconfigsToJoinMasters", "ConfigureKubeAPIServerOnJoinMasters")
	fragment.AddDependency("DistributeKubeletCertsToJoinMasters", "CreateKubeletConfigForJoinMasters")

	// Binaries must be installed before configuring services
	fragment.AddDependency("InstallKubeAPIServerOnJoinMasters", "ConfigureKubeAPIServerOnJoinMasters")
	fragment.AddDependency("InstallKubeControllerManagerOnJoinMasters", "ConfigureKubeControllerManagerOnJoinMasters")
	fragment.AddDependency("InstallKubeSchedulerOnJoinMasters", "ConfigureKubeSchedulerOnJoinMasters")
	fragment.AddDependency("InstallKubeletOnJoinMasters", "InstallKubeletServiceOnJoinMasters")
	fragment.AddDependency("InstallKubeProxyOnJoinMasters", "InstallKubeProxyServiceOnJoinMasters")

	// Configure must happen before enabling/starting services
	fragment.AddDependency("ConfigureKubeAPIServerOnJoinMasters", "InstallKubeAPIServerServiceOnJoinMasters")
	fragment.AddDependency("ConfigureKubeControllerManagerOnJoinMasters", "InstallKubeControllerManagerServiceOnJoinMasters")
	fragment.AddDependency("ConfigureKubeSchedulerOnJoinMasters", "InstallKubeSchedulerServiceOnJoinMasters")
	fragment.AddDependency("CreateKubeletConfigForJoinMasters", "InstallKubeletDropInOnJoinMasters")
	fragment.AddDependency("InstallKubeletServiceOnJoinMasters", "InstallKubeletDropInOnJoinMasters")
	fragment.AddDependency("CreateKubeProxyConfigForJoinMasters", "InstallKubeProxyServiceOnJoinMasters")

	// Service files must exist before enabling/starting
	fragment.AddDependency("InstallKubeAPIServerServiceOnJoinMasters", "EnableKubeAPIServerOnJoinMasters")
	fragment.AddDependency("InstallKubeControllerManagerServiceOnJoinMasters", "EnableKubeControllerManagerOnJoinMasters")
	fragment.AddDependency("InstallKubeSchedulerServiceOnJoinMasters", "EnableKubeSchedulerOnJoinMasters")
	fragment.AddDependency("InstallKubeletDropInOnJoinMasters", "EnableKubeletOnJoinMasters")
	fragment.AddDependency("InstallKubeProxyServiceOnJoinMasters", "EnableKubeProxyOnJoinMasters")

	// Enable before start
	fragment.AddDependency("EnableKubeAPIServerOnJoinMasters", "RestartKubeAPIServerOnJoinMasters")
	fragment.AddDependency("RestartKubeAPIServerOnJoinMasters", "CheckKubeAPIServerHealthOnJoinMasters")
	fragment.AddDependency("EnableKubeControllerManagerOnJoinMasters", "StartKubeControllerManagerOnJoinMasters")
	fragment.AddDependency("EnableKubeSchedulerOnJoinMasters", "StartKubeSchedulerOnJoinMasters")
	fragment.AddDependency("EnableKubeletOnJoinMasters", "StartKubeletOnJoinMasters")
	fragment.AddDependency("EnableKubeProxyOnJoinMasters", "StartKubeProxyOnJoinMasters")

	// APIServer must be healthy before starting controller-manager and scheduler
	fragment.AddDependency("CheckKubeAPIServerHealthOnJoinMasters", "EnableKubeControllerManagerOnJoinMasters")
	fragment.AddDependency("CheckKubeAPIServerHealthOnJoinMasters", "EnableKubeSchedulerOnJoinMasters")

	// Kubelet must start before kube-proxy
	fragment.AddDependency("StartKubeletOnJoinMasters", "StartKubeProxyOnJoinMasters")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*JoinMastersTask)(nil)
