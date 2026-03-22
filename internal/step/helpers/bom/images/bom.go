package images

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"path"
	"strings"
)

// ImageBOM 定义了一个镜像的版本及其在原始仓库的元数据。
type ImageBOM struct {
	KubeVersionConstraints string
	RepoAddr               string
	Namespace              string
	Repo                   string
	Tag                    string
}

// componentImageBOMs 是镜像的核心物料清单。
// 列表按版本从新到旧排列。
var componentImageBOMs = map[string][]ImageBOM{
	// --- K8s Core Components ---
	"pause": {
		{KubeVersionConstraints: ">= 1.26.0", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "pause", Tag: "3.9"},
		{KubeVersionConstraints: "== 1.25.x", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "pause", Tag: "3.8"},
		{KubeVersionConstraints: "== 1.24.x", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "pause", Tag: "3.7"},
		{KubeVersionConstraints: "< 1.24.0", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "pause", Tag: "3.6"},
	},
	"etcd": {
		{KubeVersionConstraints: ">= 1.29.0", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "etcd", Tag: "3.5.10-0"},
		{KubeVersionConstraints: "< 1.29.0", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "etcd", Tag: "3.5.9-0"},
	},
	"coredns": {
		{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "registry.k8s.io", Namespace: "coredns", Repo: "coredns", Tag: "v1.10.1"},
		{KubeVersionConstraints: "< 1.28.0", RepoAddr: "registry.k8s.io", Namespace: "coredns", Repo: "coredns", Tag: "v1.9.3"},
	},
	"k8s-dns-node-cache": {
		{KubeVersionConstraints: ">= 1.22.0", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "k8s-dns-node-cache", Tag: "1.22.20"},
	},
	"conformance": {
		{KubeVersionConstraints: ">= 1.29.0", RepoAddr: "registry.k8s.io", Namespace: "", Repo: "conformance", Tag: "v1.29.0"},
	},

	// --- Calico (Versions should align with Helm chart versions) ---
	"tigera-operator":             {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "tigera", Repo: "operator", Tag: "v1.32.7"}},
	"calico-cni":                  {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "cni", Tag: "v3.28.0"}},
	"calico-node":                 {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "node", Tag: "v3.28.0"}},
	"calico-kube-controllers":     {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "kube-controllers", Tag: "v3.28.0"}},
	"calico-apiserver":            {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "apiserver", Tag: "v3.28.0"}},
	"calico-typha":                {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "typha", Tag: "v3.28.0"}},
	"calico-flexvol":              {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "pod2daemon-flexvol", Tag: "v3.28.0"}},
	"calico-key-cert-provisioner": {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "key-cert-provisioner", Tag: "v3.28.0"}},
	"calico-dikastes":             {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "dikastes", Tag: "v3.28.0"}},
	"calico-envoy-gateway":        {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "envoy", Tag: "v3.28.0-envoy"}},
	"calico-envoy-proxy":          {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "envoy", Tag: "v3.28.0-envoy"}},
	"calico-envoy-ratelimit":      {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "ratelimit", Tag: "v3.28.0-ratelimit"}},
	"calico-goldmane":             {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "goldmane", Tag: "v3.28.0"}},
	"calico-whisker":              {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "whisker", Tag: "v3.28.0"}},
	"calico-whisker-backend":      {ImageBOM{KubeVersionConstraints: ">= 1.28.0", RepoAddr: "quay.io", Namespace: "calico", Repo: "whisker-backend", Tag: "v3.28.0"}},

	// --- Flannel ---
	"flannel":            {ImageBOM{KubeVersionConstraints: ">= 1.22.0", RepoAddr: "quay.io", Namespace: "flannel", Repo: "flannel", Tag: "v0.25.5"}},
	"flannel-cni-plugin": {ImageBOM{KubeVersionConstraints: ">= 1.24.0", RepoAddr: "quay.io", Namespace: "flannel", Repo: "flannel-cni-plugin", Tag: "v1.4.1"}},

	// --- Cilium ---
	"cilium":                  {ImageBOM{KubeVersionConstraints: ">= 1.27.0", RepoAddr: "quay.io", Namespace: "cilium", Repo: "cilium", Tag: "v1.15.7"}},
	"cilium-operator-generic": {ImageBOM{KubeVersionConstraints: ">= 1.27.0", RepoAddr: "quay.io", Namespace: "cilium", Repo: "operator-generic", Tag: "v1.15.7"}},

	// --- Kube-OVN ---
	"kubeovn": {ImageBOM{KubeVersionConstraints: ">= 1.26.0", RepoAddr: "ghcr.io", Namespace: "kubeovn", Repo: "kube-ovn", Tag: "v1.13.1"}},

	// --- Hybridnet ---
	"hybridnet": {ImageBOM{KubeVersionConstraints: ">= 1.23.0", RepoAddr: "ghcr.io", Namespace: "alibaba", Repo: "hybridnet", Tag: "v0.8.4"}},

	// --- Multus ---
	"multus": {ImageBOM{KubeVersionConstraints: ">= 1.22.0", RepoAddr: "ghcr.io", Namespace: "k8snetworkplumbingwg", Repo: "multus-cni", Tag: "v5.0.1"}},

	// --- Storage ---
	"provisioner-localpv":       {ImageBOM{KubeVersionConstraints: ">= 1.23.0", RepoAddr: "quay.io", Namespace: "openebs", Repo: "provisioner-localpv", Tag: "4.0.1"}},
	"linux-utils":               {ImageBOM{KubeVersionConstraints: ">= 1.23.0", RepoAddr: "quay.io", Namespace: "openebs", Repo: "linux-utils", Tag: "4.0.1"}},
	"nfs-plugin":                {ImageBOM{KubeVersionConstraints: ">= 1.22.0", RepoAddr: "registry.k8s.io", Namespace: "sig-storage", Repo: "nfs-subdir-external-provisioner", Tag: "v4.0.2"}},
	"csi-provisioner":           {ImageBOM{KubeVersionConstraints: ">= 1.25.0", RepoAddr: "registry.k8s.io", Namespace: "sig-storage", Repo: "csi-provisioner", Tag: "v4.0.1"}},
	"csi-node-driver-registrar": {ImageBOM{KubeVersionConstraints: ">= 1.25.0", RepoAddr: "registry.k8s.io", Namespace: "sig-storage", Repo: "csi-node-driver-registrar", Tag: "v2.10.1"}},
	"csi-resizer":               {ImageBOM{KubeVersionConstraints: ">= 1.25.0", RepoAddr: "registry.k8s.io", Namespace: "sig-storage", Repo: "csi-resizer", Tag: "v1.10.1"}},
	"csi-snapshotter":           {ImageBOM{KubeVersionConstraints: ">= 1.25.0", RepoAddr: "registry.k8s.io", Namespace: "sig-storage", Repo: "csi-snapshotter", Tag: "v7.0.2"}},

	// --- Load Balancer & Infra ---
	"haproxy": {ImageBOM{KubeVersionConstraints: ">= 0.0.0", RepoAddr: "docker.io", Namespace: "library", Repo: "haproxy", Tag: "2.9.7-alpine"}},
	"nginx":   {ImageBOM{KubeVersionConstraints: ">= 0.0.0", RepoAddr: "docker.io", Namespace: "library", Repo: "nginx", Tag: "1.25.5-alpine"}},
	"kubevip": {ImageBOM{KubeVersionConstraints: ">= 0.0.0", RepoAddr: "ghcr.io", Namespace: "kube-vip", Repo: "kube-vip", Tag: "v0.8.0"}},

	// --- Addons ---
	"kata-deploy":            {ImageBOM{KubeVersionConstraints: ">= 1.24.0", RepoAddr: "quay.io", Namespace: "kata-containers", Repo: "kata-deploy", Tag: "stable"}},
	"node-feature-discovery": {ImageBOM{KubeVersionConstraints: ">= 1.24.0", RepoAddr: "registry.k8s.io", Namespace: "nfd", Repo: "node-feature-discovery", Tag: "v0.15.2"}},
}

// getImageBOM 是一个内部函数，用于从BOM中获取最匹配的镜像信息。
// 它处理版本约束和版本择优。
func getImageBOM(componentName string, kubeVersionStr string) *ImageBOM {
	if componentName == "" || kubeVersionStr == "" {
		return nil
	}
	if !strings.HasPrefix(kubeVersionStr, "v") {
		kubeVersionStr = "v" + kubeVersionStr
	}
	k8sVersion, err := semver.NewVersion(kubeVersionStr)
	if err != nil {
		fmt.Printf("Warning: could not parse kubernetes version '%s' for image BOM lookup: %v\n", kubeVersionStr, err)
		return nil
	}
	bomList, ok := componentImageBOMs[componentName]
	if !ok {
		switch componentName {
		case "kube-apiserver", "kube-controller-manager", "kube-scheduler", "kube-proxy":
			return &ImageBOM{
				KubeVersionConstraints: kubeVersionStr,
				RepoAddr:               "registry.k8s.io",
				Namespace:              "",
				Repo:                   componentName,
				Tag:                    kubeVersionStr,
			}
		}
		return nil
	}
	var candidates []ImageBOM
	for _, bomEntry := range bomList {
		constraintStr := strings.ReplaceAll(bomEntry.KubeVersionConstraints, ".x", ".*")
		constraints, err := semver.NewConstraint(constraintStr)
		if err != nil {
			fmt.Printf("Error: invalid version constraint in image BOM for %s: '%s'. Skipping.\n", componentName, bomEntry.KubeVersionConstraints)
			continue
		}
		if constraints.Check(k8sVersion) {
			candidates = append(candidates, bomEntry)
		}
	}
	if len(candidates) == 0 {
		fmt.Printf("Warning: no compatible image version found in BOM for component '%s' with K8s '%s'\n", componentName, kubeVersionStr)
		return nil
	}
	bestCandidate := candidates[0]
	bestVersion, err := semver.NewVersion(bestCandidate.Tag)
	if err != nil {
		bestVersion = nil // Mark as unparsable
	}
	for i := 1; i < len(candidates); i++ {
		currentVersion, err := semver.NewVersion(candidates[i].Tag)
		if err != nil {
			continue // Skip unparsable tags like "stable"
		}
		if bestVersion == nil || currentVersion.GreaterThan(bestVersion) {
			bestVersion = currentVersion
			bestCandidate = candidates[i]
		}
	}
	bomCopy := bestCandidate
	return &bomCopy
}

var refToComponentIndex map[string]string

// init 函数在包加载时执行，用于自动构建反向索引。
func init() {
	refToComponentIndex = make(map[string]string)

	// 遍历BOM，为每个组件的所有已知镜像引用创建索引
	for componentName, bomList := range componentImageBOMs {
		for _, bom := range bomList {
			// 构建引用路径, e.g., "quay.io/calico/node"
			var ref string
			if bom.Namespace == "" {
				ref = path.Join(bom.RepoAddr, bom.Repo)
			} else {
				ref = path.Join(bom.RepoAddr, bom.Namespace, bom.Repo)
			}

			// 如果索引中还没有这个引用，或者需要覆盖（取决于你的策略），则添加它
			if _, exists := refToComponentIndex[ref]; !exists {
				refToComponentIndex[ref] = componentName
			}
		}
	}

	// 手动添加 K8s 核心组件到索引中
	k8sCoreRepo := "registry.k8s.io"
	refToComponentIndex[path.Join(k8sCoreRepo, "kube-apiserver")] = "kube-apiserver"
	refToComponentIndex[path.Join(k8sCoreRepo, "kube-controller-manager")] = "kube-controller-manager"
	refToComponentIndex[path.Join(k8sCoreRepo, "kube-scheduler")] = "kube-scheduler"
	refToComponentIndex[path.Join(k8sCoreRepo, "kube-proxy")] = "kube-proxy"
}

// GetComponentNameByRef 是新的、供外部调用的反向查找函数。
// 它根据一个镜像的引用字符串（不含标签），查找其在BOM中的组件名。
//
// 参数:
//
//	ref: 镜像的引用字符串, e.g., "quay.io/calico/node"
//
// 返回:
//
//	对应的组件名 (e.g., "calico-node")，如果找不到则返回空字符串。
func GetComponentNameByRef(ref string) string {
	return refToComponentIndex[ref]
}
