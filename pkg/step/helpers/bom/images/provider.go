package images

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"strings"
)

// ImageProvider 封装了所有与获取镜像信息相关的逻辑。
// 它是外部代码（如 Steps）与镜像BOM和类型交互的唯一入口。
type ImageProvider struct {
	ctx runtime.ExecutionContext
}

// NewImageProvider 创建一个新的镜像提供者实例。
func NewImageProvider(ctx runtime.ExecutionContext) *ImageProvider {
	return &ImageProvider{ctx: ctx}
}

// GetImage 获取指定组件的单个镜像对象。
// 它处理版本推断、启用/禁用逻辑和私有仓库重写。
// 如果组件被禁用或在BOM中找不到，将返回 nil。
func (p *ImageProvider) GetImage(name string) *Image {
	cfg := p.ctx.GetClusterConfig()
	if cfg == nil {
		return nil // 如果配置不存在，无法继续
	}
	kubeVersionStr := cfg.Spec.Kubernetes.Version

	// 1. 从BOM获取基础镜像元数据
	bom := getImageBOM(name, kubeVersionStr)
	if bom == nil {
		return nil // BOM中没有此镜像或版本不兼容
	}

	// 2. 判断该组件是否在当前配置下启用
	if !p.isImageEnabled(name) {
		return nil
	}

	// 3. 使用 newImage 工厂函数创建并返回一个完整的 Image 对象
	return newImage(bom, cfg.Spec.Registry.MirroringAndRewriting)
}

// GetImages 获取当前配置下所有已启用的镜像对象列表。
// 这是 `SaveImagesStep` 等批量操作应该调用的主要函数。
func (p *ImageProvider) GetImages() []*Image {
	var enabledImages []*Image
	allImageNames := p.getManagedImageNames()

	for _, name := range allImageNames {
		if image := p.GetImage(name); image != nil {
			enabledImages = append(enabledImages, image)
		}
	}
	return enabledImages
}

// getManagedImageNames 返回BOM中管理的所有镜像组件名称列表。
func (p *ImageProvider) getManagedImageNames() []string {
	// 从 componentImageBOMs 的 keys 动态生成
	names := make([]string, 0, len(componentImageBOMs))
	for name := range componentImageBOMs {
		names = append(names, name)
	}
	// 加上那些不由BOM管理但需要硬编码处理的核心组件
	names = append(names, "kube-apiserver", "kube-controller-manager", "kube-scheduler", "kube-proxy")
	return names
}

// isImageEnabled 封装了判断镜像是否启用的逻辑。
func (p *ImageProvider) isImageEnabled(name string) bool {
	cfg := p.ctx.GetClusterConfig().Spec

	// 具体的启用/禁用判断
	switch name {
	case "etcd":
		return strings.EqualFold(cfg.Etcd.Type, string(common.EtcdDeploymentTypeKubeadm))
	case "kube-proxy":
		return cfg.Kubernetes.KubeProxy.Enable == nil || *cfg.Kubernetes.KubeProxy.Enable
	case "k8s-dns-node-cache":
		return cfg.DNS.NodeLocalDNS != nil && cfg.DNS.NodeLocalDNS.Enabled != nil && *cfg.DNS.NodeLocalDNS.Enabled

	// CNI Images
	case "tigera-operator", "calico-cni", "calico-node", "calico-kube-controllers", "calico-apiserver", "calico-typha", "calico-flexvol", "calico-key-cert-provisioner", "calico-dikastes", "calico-envoy-gateway", "calico-envoy-proxy", "calico-envoy-ratelimit", "calico-goldmane", "calico-whisker", "calico-whisker-backend":
		return strings.EqualFold(cfg.Network.Plugin, string(common.CNITypeCalico))
	case "flannel", "flannel-cni-plugin":
		return strings.EqualFold(cfg.Network.Plugin, string(common.CNITypeFlannel))
	case "cilium", "cilium-operator-generic":
		return strings.EqualFold(cfg.Network.Plugin, string(common.CNITypeCilium))
	case "kubeovn":
		return strings.EqualFold(cfg.Network.Plugin, string(common.CNITypeKubeOvn))
	case "hybridnet":
		return strings.EqualFold(cfg.Network.Plugin, string(common.CNITypeHybridnet))
	case "multus":
		return cfg.Network.Multus != nil && cfg.Network.Multus.Installation.Enabled != nil && *cfg.Network.Multus.Installation.Enabled

	// Storage Images
	case "provisioner-localpv", "linux-utils":
		return cfg.Storage.OpenEBS != nil && cfg.Storage.OpenEBS.Enabled != nil && *cfg.Storage.OpenEBS.Enabled
	case "nfs-plugin", "csi-provisioner", "csi-node-driver-registrar", "csi-resizer", "csi-snapshotter":
		return cfg.Storage.NFS != nil && cfg.Storage.NFS.Enabled != nil && *cfg.Storage.NFS.Enabled

	// Load Balancer Images
	case "haproxy":
		return cfg.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeHAProxy
	case "nginx":
		return cfg.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeNginx
	case "kubevip":
		return cfg.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeKubeVIP

	// Addon Images
	case "kata-deploy":
		return cfg.Kubernetes.Addons.Kata != nil && cfg.Kubernetes.Addons.Kata.Enabled != nil && *cfg.Kubernetes.Addons.Kata.Enabled
	case "node-feature-discovery":
		return cfg.Kubernetes.Addons.NodeFeatureDiscovery != nil && cfg.Kubernetes.Addons.NodeFeatureDiscovery.Enabled != nil && *cfg.Kubernetes.Addons.NodeFeatureDiscovery.Enabled

	default:
		// 对于 pause, conformance, kube-apiserver 等核心组件，总是启用
		return true
	}
}
