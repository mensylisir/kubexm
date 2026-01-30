package loadbalancer

// ===================================================================
// LoadBalancer Task Factory - 任务工厂
// 用于根据配置自动选择正确的 LoadBalancer Tasks
// ===================================================================

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/loadbalancer/haproxy"
	kubevip "github.com/mensylisir/kubexm/pkg/task/loadbalancer/kube-vip"
	"github.com/mensylisir/kubexm/pkg/task/loadbalancer/nginx"
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
			tasks = append(tasks, kube-vip.NewDeployKubeVipTask())
		}
	}

	// Internal LoadBalancer (Static Pod 或 Daemon)
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal != nil {
		internalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type

		switch internalType {
		case string(common.InternalLBTypeHAProxy):
			// HAProxy Static Pod (kubeadm) 或 Daemon (kubexm)
			if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
				tasks = append(tasks, haproxy.NewDeployHAProxyAsDaemonTask())
			} else {
				tasks = append(tasks, haproxy.NewDeployHAProxyAsStaticPodTask())
			}
		case string(common.InternalLBTypeNginx):
			// Nginx Static Pod (kubeadm) 或 Daemon (kubexm)
			if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
				tasks = append(tasks, nginx.NewDeployNginxAsDaemonTask())
			} else {
				tasks = append(tasks, nginx.NewDeployNginxAsStaticPodTask())
			}
		case string(common.InternalLBTypeKubeVIP):
			tasks = append(tasks, kubevip.NewDeployKubeVipTask())
		}
	}

	return tasks
}
