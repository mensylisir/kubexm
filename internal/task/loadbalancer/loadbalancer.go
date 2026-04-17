package loadbalancer

// ===================================================================
// LoadBalancer Task Factory - 任务工厂
// 用于根据配置自动选择正确的 LoadBalancer Tasks
// ===================================================================

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/task/loadbalancer/haproxy"
	kubevip "github.com/mensylisir/kubexm/internal/task/loadbalancer/kube-vip"
	"github.com/mensylisir/kubexm/internal/task/loadbalancer/nginx"
)

// GetLoadBalancerTasks 根据配置返回需要执行的 LoadBalancer Tasks
// 返回的 Tasks 已经按照正确的依赖顺序排列
func GetLoadBalancerTasks(ctx runtime.TaskContext) []task.Task {
	cfg := ctx.GetClusterConfig()
	var tasks []task.Task

	// 判断 HA 模式是否启用
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
		return tasks
	}

	// External LoadBalancer (Keepalived + HAProxy/Nginx)
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.External != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled != nil &&
		*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {

		externalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type

		switch externalType {
		case string(common.ExternalLBTypeKubexmKH):
			// kubexm-kh: Keepalived + HAProxy
			tasks = append(tasks, NewInstallKeepalivedTask())
			tasks = append(tasks, haproxy.NewDeployHAProxyAsDaemonTask())
		case string(common.ExternalLBTypeKubexmKN):
			// kubexm-kn: Keepalived + Nginx
			tasks = append(tasks, NewInstallKeepalivedTask())
			tasks = append(tasks, nginx.NewDeployNginxAsDaemonTask())
		case string(common.ExternalLBTypeKubeVIP):
			// kube-vip: 独立部署
			tasks = append(tasks, kubevip.NewDeployKubeVipTask())
		case string(common.ExternalLBTypeExternal):
			// external/exists: skip deployment
			return tasks
		}
	}

	// Internal LoadBalancer (Static Pod 或 Daemon)
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled != nil &&
		*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled {
		internalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type

		switch internalType {
		case string(common.InternalLBTypeHAProxy):
			// HAProxy Static Pod (kubeadm) 或 Daemon (kubexm)
			if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
				tasks = append(tasks, haproxy.NewDeployHAProxyOnWorkersTask())
			} else {
				tasks = append(tasks, haproxy.NewDeployHAProxyAsStaticPodTask())
			}
		case string(common.InternalLBTypeNginx):
			// Nginx Static Pod (kubeadm) 或 Daemon (kubexm)
			if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
				tasks = append(tasks, nginx.NewDeployNginxOnWorkersTask())
			} else {
				tasks = append(tasks, nginx.NewDeployNginxAsStaticPodTask())
			}
		case string(common.InternalLBTypeKubeVIP):
			tasks = append(tasks, kubevip.NewDeployKubeVipTask())
		}
	}

	return tasks
}

// GetLoadBalancerCleanupTasks returns the appropriate cleanup tasks for the configured load balancer type.
func GetLoadBalancerCleanupTasks(ctx runtime.TaskContext) []task.Task {
	cfg := ctx.GetClusterConfig()
	var tasks []task.Task

	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
		return tasks
	}

	// External LoadBalancer cleanup
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.External != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled != nil &&
		*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {

		externalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type

		switch externalType {
		case string(common.ExternalLBTypeKubexmKH):
			tasks = append(tasks, NewUninstallHAProxyAsDaemonTask())
			tasks = append(tasks, NewUninstallKeepalivedTask())
		case string(common.ExternalLBTypeKubexmKN):
			tasks = append(tasks, nginx.NewCleanNginxAsDaemonTask())
			tasks = append(tasks, NewUninstallKeepalivedTask())
		case string(common.ExternalLBTypeKubeVIP):
			tasks = append(tasks, kubevip.NewCleanKubeVipTask())
		case string(common.ExternalLBTypeExternal):
			return tasks
		}
	}

	// Internal LoadBalancer cleanup
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal != nil &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled != nil &&
		*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled {
		internalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type

		switch internalType {
		case string(common.InternalLBTypeHAProxy):
			// Cleanup static pod (kubeadm) or worker daemon (kubexm)
			if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
				tasks = append(tasks, haproxy.NewCleanHAProxyOnWorkersTask())
			} else {
				tasks = append(tasks, haproxy.NewCleanHAProxyStaticPodTask())
			}
		case string(common.InternalLBTypeNginx):
			// Cleanup static pod (kubeadm) or worker daemon (kubexm)
			if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
				tasks = append(tasks, nginx.NewCleanNginxOnWorkersTask())
			} else {
				tasks = append(tasks, nginx.NewCleanNginxStaticPodTask())
			}
		case string(common.InternalLBTypeKubeVIP):
			tasks = append(tasks, kubevip.NewCleanKubeVipTask())
		}
	}

	return tasks
}
