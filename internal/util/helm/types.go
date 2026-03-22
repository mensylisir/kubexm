package helm

import (
	"fmt"
	"path"
	"strings"
)

// HelmChart 结构体代表一个 Helm Chart 的所有相关信息。
// 这是一个自包含的结构，内置了所有名称处理和版本管理的逻辑。
type HelmChart struct {
	// --- 核心属性 ---
	ComponentName string // e.g., "calico", "multus"
	Version       string // The exact version of the chart, e.g., "v3.28.0"
	KubeVersion   string // The Kubernetes version this chart was resolved for

	// --- 元数据 (从 BOM 获取) ---
	meta ChartInfo

	// --- 配置 (用于动态决策) ---
	// 这里可以添加从 ClusterConfig 中传入的、影响 Chart 行为的特定配置
	// 例如: privateRegistry string
}

// ChartInfo 是在 helm_bom.go 中定义的、存储BOM元数据的基础结构。
type ChartInfo struct {
	Name    string
	Repo    string
	Version string
}

// --- 公共方法 ---

// ChartName 返回 Chart 的名称, e.g., "tigera-operator"
func (h *HelmChart) ChartName() string {
	return h.meta.Name
}

// RepoName 返回 Chart 的仓库别名, 通常就是组件名, e.g., "calico"
func (h *HelmChart) RepoName() string {
	// 我们可以约定仓库别名就是组件名的小写形式
	return strings.ToLower(h.ComponentName)
}

// RepoURL 返回 Chart 的仓库 URL。
func (h *HelmChart) RepoURL() string {
	return h.meta.Repo
}

// FullName 返回用于 `helm install` 的完整 Chart 名称, e.g., "calico/tigera-operator"
func (h *HelmChart) FullName() string {
	return fmt.Sprintf("%s/%s", h.RepoName(), h.ChartName())
}

// LocalPath 返回 Chart 在本地离线包中应该存放的路径。
// e.g., ".kubexm/helm/v1.28.5/calico/tigera-operator-v3.28.0.tgz"
func (h *HelmChart) LocalPath(baseDir string) string {
	chartFileName := fmt.Sprintf("%s-%s.tgz", h.ChartName(), h.Version)
	return path.Join(baseDir, "helm", h.KubeVersion, h.RepoName(), chartFileName)
}
