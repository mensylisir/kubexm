package loadbalancer

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/loadbalancer/haproxy"
	lbpkg "github.com/mensylisir/kubexm/pkg/step/loadbalancer/keepalived"
	kubevippkg "github.com/mensylisir/kubexm/pkg/step/loadbalancer/kube-vip"
	"github.com/mensylisir/kubexm/pkg/task"
)

// ===================================================================
// Keepalived Tasks - 完整的生命周期
// ===================================================================

// InstallKeepalivedTask 安装Keepalived (systemd)
// 组合: InstallPackage → GenerateConfig → DeployConfig → Enable → Start
type InstallKeepalivedTask struct {
	task.Base
}

func NewInstallKeepalivedTask() task.Task {
	return &InstallKeepalivedTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallKeepalived",
				Description: "Install Keepalived as systemd service",
			},
		},
	}
}

func (t *InstallKeepalivedTask) Name() string        { return t.Meta.Name }
func (t *InstallKeepalivedTask) Description() string { return t.Meta.Description }

func (t *InstallKeepalivedTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {
		return false, nil
	}
	lbType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type
	return lbType == string(common.ExternalLBTypeKubexmKH) ||
		lbType == string(common.ExternalLBTypeKubexmKN), nil
}

func (t *InstallKeepalivedTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.(*runtime.Context).ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// 组合Step: InstallPackage → GenerateConfig → DeployConfig → Enable → Start
	installPkg := lbpkg.NewInstallKeepalivedPackage(*execCtx, "InstallKeepalivedPackage")
	generateCfg := lbpkg.NewGenerateKeepalivedConfig(*execCtx, "GenerateKeepalivedConfig")
	deployCfg := lbpkg.NewDeployKeepalivedConfig(*execCtx, "DeployKeepalivedConfig")
	enableSvc := lbpkg.NewEnableKeepalivedService(*execCtx, "EnableKeepalivedService")
	startSvc := lbpkg.NewStartKeepalivedService(*execCtx, "StartKeepalivedService")

	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKeepalivedPackage", Step: installPkg, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKeepalivedConfig", Step: generateCfg, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DeployKeepalivedConfig", Step: deployCfg, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKeepalivedService", Step: enableSvc, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKeepalivedService", Step: startSvc, Hosts: hosts})

	fragment.AddDependency("InstallKeepalivedPackage", "GenerateKeepalivedConfig")
	fragment.AddDependency("GenerateKeepalivedConfig", "DeployKeepalivedConfig")
	fragment.AddDependency("DeployKeepalivedConfig", "EnableKeepalivedService")
	fragment.AddDependency("EnableKeepalivedService", "StartKeepalivedService")
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// UninstallKeepalivedTask 卸载Keepalived
// 组合: Stop → Disable → RemovePackage → RemoveConfig
type UninstallKeepalivedTask struct {
	task.Base
}

func NewUninstallKeepalivedTask() task.Task {
	return &UninstallKeepalivedTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "UninstallKeepalived",
				Description: "Uninstall Keepalived service",
			},
		},
	}
}

func (t *UninstallKeepalivedTask) Name() string        { return t.Meta.Name }
func (t *UninstallKeepalivedTask) Description() string { return t.Meta.Description }

func (t *UninstallKeepalivedTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return (&InstallKeepalivedTask{}).IsRequired(ctx)
}

func (t *UninstallKeepalivedTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.(*runtime.Context).ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// 组合Step: Stop → Disable → RemovePackage → RemoveConfig
	stopSvc := lbpkg.NewStopKeepalivedService(*execCtx, "StopKeepalivedService")
	disableSvc := lbpkg.NewDisableKeepalivedService(*execCtx, "DisableKeepalivedService")
	removePkg := lbpkg.NewRemoveKeepalivedPackage(*execCtx, "RemoveKeepalivedPackage")
	removeCfg := lbpkg.NewRemoveKeepalivedConfig(*execCtx, "RemoveKeepalivedConfig")

	fragment.AddNode(&plan.ExecutionNode{Name: "StopKeepalivedService", Step: stopSvc, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableKeepalivedService", Step: disableSvc, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveKeepalivedPackage", Step: removePkg, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveKeepalivedConfig", Step: removeCfg, Hosts: hosts})

	fragment.AddDependency("StopKeepalivedService", "DisableKeepalivedService")
	fragment.AddDependency("DisableKeepalivedService", "RemoveKeepalivedPackage")
	fragment.AddDependency("RemoveKeepalivedPackage", "RemoveKeepalivedConfig")
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// ===================================================================
// HAProxy Tasks - 生命周期完整
// ===================================================================

// InstallHAProxyAsDaemonTask 安装HAProxy (systemd)
type InstallHAProxyAsDaemonTask struct {
	task.Base
}

func NewInstallHAProxyAsDaemonTask() task.Task {
	return &InstallHAProxyAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallHAProxyAsDaemon",
				Description: "Install HAProxy as systemd service",
			},
		},
	}
}

func (t *InstallHAProxyAsDaemonTask) Name() string        { return t.Meta.Name }
func (t *InstallHAProxyAsDaemonTask) Description() string { return t.Meta.Description }

func (t *InstallHAProxyAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
		return false, nil
	}

	// external kubexm-kh 或 internal kubexm + haproxy
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.External != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled != nil &&
		*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {
		return cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type == string(common.ExternalLBTypeKubexmKH), nil
	}

	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
		return cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal != nil &&
			cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type == string(common.InternalLBTypeHAProxy), nil
	}

	return false, nil
}

func (t *InstallHAProxyAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.(*runtime.Context).ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// 组合Step: InstallHAProxy → GenerateHAProxyConfig → Enable → Start
	installHAProxy, _ := haproxy.NewInstallHAProxyStepBuilder(*execCtx, "InstallHAProxy").Build()
	generateHAProxyConfig, _ := haproxy.NewGenerateHAProxyConfigStepBuilder(*execCtx, "GenerateHAProxyConfig").Build()
	enableHAProxy, _ := haproxy.NewEnableHAProxyStepBuilder(*execCtx, "EnableHAProxyService").Build()
	startHAProxy, _ := haproxy.NewStartHAProxyStepBuilder(*execCtx, "StartHAProxyService").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "InstallHAProxy", Step: installHAProxy, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHAProxyConfig", Step: generateHAProxyConfig, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableHAProxyService", Step: enableHAProxy, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartHAProxyService", Step: startHAProxy, Hosts: hosts})

	fragment.AddDependency("InstallHAProxy", "GenerateHAProxyConfig")
	fragment.AddDependency("GenerateHAProxyConfig", "EnableHAProxyService")
	fragment.AddDependency("EnableHAProxyService", "StartHAProxyService")
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// UninstallHAProxyAsDaemonTask 卸载HAProxy
type UninstallHAProxyAsDaemonTask struct {
	task.Base
}

func NewUninstallHAProxyAsDaemonTask() task.Task {
	return &UninstallHAProxyAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "UninstallHAProxyAsDaemon",
				Description: "Uninstall HAProxy service",
			},
		},
	}
}

func (t *UninstallHAProxyAsDaemonTask) Name() string        { return t.Meta.Name }
func (t *UninstallHAProxyAsDaemonTask) Description() string { return t.Meta.Description }

func (t *UninstallHAProxyAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return (&InstallHAProxyAsDaemonTask{}).IsRequired(ctx)
}

func (t *UninstallHAProxyAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.(*runtime.Context).ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// 组合Step: Stop → Disable → CleanHAProxy
	stopHAProxy, _ := haproxy.NewStopHAProxyStepBuilder(*execCtx, "StopHAProxyService").Build()
	disableHAProxy, _ := haproxy.NewDisableHAProxyStepBuilder(*execCtx, "DisableHAProxyService").Build()
	cleanHAProxy, _ := haproxy.NewCleanHAProxyStepBuilder(*execCtx, "CleanHAProxy").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "StopHAProxyService", Step: stopHAProxy, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableHAProxyService", Step: disableHAProxy, Hosts: hosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CleanHAProxy", Step: cleanHAProxy, Hosts: hosts})

	fragment.AddDependency("StopHAProxyService", "DisableHAProxyService")
	fragment.AddDependency("DisableHAProxyService", "CleanHAProxy")
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// ===================================================================
// Kube-VIP Tasks
// ===================================================================

// DeployKubeVIPTask 部署Kube-VIP
type DeployKubeVIPTask struct {
	task.Base
}

func NewDeployKubeVIPTask() task.Task {
	return &DeployKubeVIPTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployKubeVIP",
				Description: "Deploy Kube-VIP as static pod",
			},
		},
	}
}

func (t *DeployKubeVIPTask) Name() string        { return t.Meta.Name }
func (t *DeployKubeVIPTask) Description() string { return t.Meta.Description }

func (t *DeployKubeVIPTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
		return false, nil
	}

	lbType := ""
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.External != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled != nil &&
		*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {
		lbType = cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type
	}
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != "" {
		lbType = cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type
	}

	return lbType == string(common.InternalLBTypeKubeVIP) ||
		lbType == string(common.ExternalLBTypeKubeVIP), nil
}

func (t *DeployKubeVIPTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.(*runtime.Context).ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// 组合Step: GenerateKubeVIPManifest
	generateKubeVIP, _ := kubevippkg.NewGenerateKubeVipManifestStepBuilder(*execCtx, "GenerateKubeVIPManifest").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeVIPManifest", Step: generateKubeVIP, Hosts: hosts})
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// CleanupKubeVIPTask 清理Kube-VIP
type CleanupKubeVIPTask struct {
	task.Base
}

func NewCleanupKubeVIPTask() task.Task {
	return &CleanupKubeVIPTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanupKubeVIP",
				Description: "Cleanup Kube-VIP static pod",
			},
		},
	}
}

func (t *CleanupKubeVIPTask) Name() string        { return t.Meta.Name }
func (t *CleanupKubeVIPTask) Description() string { return t.Meta.Description }

func (t *CleanupKubeVIPTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return (&DeployKubeVIPTask{}).IsRequired(ctx)
}

func (t *CleanupKubeVIPTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.(*runtime.Context).ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleControlPlane)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// 组合Step: CleanKubeVIPManifest
	cleanKubeVIP, _ := kubevippkg.NewCleanKubeVipManifestStepBuilder(*execCtx, "CleanKubeVIPManifest").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanKubeVIPManifest", Step: cleanKubeVIP, Hosts: hosts})
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
