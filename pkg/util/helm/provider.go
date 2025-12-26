package helm

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// HelmProvider 封装了所有与获取 Helm Chart 信息相关的业务逻辑。
type HelmProvider struct {
	ctx runtime.ExecutionContext
}

// NewHelmProvider 创建一个新的 Helm Chart 提供者实例。
func NewHelmProvider(ctx runtime.ExecutionContext) *HelmProvider {
	return &HelmProvider{ctx: ctx}
}

// GetChart 获取指定组件的单个 Helm Chart 对象。
// 它处理版本推断和启用/禁用逻辑。
// 如果组件被禁用或在BOM中找不到，将返回 nil。
func (p *HelmProvider) GetChart(componentName string) *HelmChart {
	cfg := p.ctx.GetClusterConfig()
	if cfg == nil {
		return nil // 如果配置不存在，无法继续
	}
	kubeVersionStr := cfg.Spec.Kubernetes.Version

	// 1. 从BOM获取基础 Chart 信息
	// **修正**: 调用 helm.GetChartInfo，这是您已经定义好的函数
	bomChartInfo := GetChartInfo(componentName, kubeVersionStr)
	if bomChartInfo == nil {
		return nil // BOM中没有此组件或版本不兼容
	}

	// 2. 判断该组件是否在当前配置下启用
	if !p.isChartEnabled(componentName) {
		return nil
	}

	// 3. 创建并返回一个完整的 HelmChart 对象
	return &HelmChart{
		ComponentName: componentName,
		Version:       bomChartInfo.Version,
		KubeVersion:   kubeVersionStr,
		meta:          *bomChartInfo, // 注意这里 bomChartInfo 是指针，需要解引用
	}
}

// GetCharts 获取当前配置下所有已启用的 Helm Chart 对象列表。
func (p *HelmProvider) GetCharts() []*HelmChart {
	var enabledCharts []*HelmChart
	// **修正**: 从 helm 包中获取组件列表
	allChartNames := GetManagedChartNames()

	for _, name := range allChartNames {
		if chart := p.GetChart(name); chart != nil {
			enabledCharts = append(enabledCharts, chart)
		}
	}
	return enabledCharts
}

// isChartEnabled 封装了判断 Chart 是否启用的逻辑。
func (p *HelmProvider) isChartEnabled(componentName string) bool {
	cfg := p.ctx.GetClusterConfig().Spec

	switch componentName {
	// CNI 插件
	case string(common.CNITypeCalico):
		return cfg.Network.Plugin == string(common.CNITypeCalico)
	case string(common.CNITypeFlannel):
		return cfg.Network.Plugin == string(common.CNITypeFlannel)
	case string(common.CNITypeCilium):
		return cfg.Network.Plugin == string(common.CNITypeCilium)
	case string(common.CNITypeKubeOvn):
		return cfg.Network.Plugin == string(common.CNITypeKubeOvn)
	case string(common.CNITypeHybridnet):
		return cfg.Network.Plugin == string(common.CNITypeHybridnet)
	case string(common.CNITypeMultus):
		return cfg.Network.Multus != nil && cfg.Network.Multus.Installation.Enabled != nil && *cfg.Network.Multus.Installation.Enabled

	// 其他组件
	case "ingress-nginx", "longhorn", "openebs", "nfs-subdir-external-provisioner", "csi-driver-nfs", "argocd":
		// 这里的启用逻辑需要根据您最终的 API spec 来确定
		// 作为一个健壮的默认，我们可以假设如果用户没有明确禁用，就是启用
		return true

	default:
		return false
	}
}
