package util

import (
	"sort"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	versionutil "k8s.io/apimachinery/pkg/util/version"
)

var k8sVersionToPauseTag = map[string]string{
	"1.30.0":  "3.9",
	"1.29.0":  "3.9",
	"1.28.0":  "3.9",
	"1.27.0":  "3.9",
	"1.26.0":  "3.9",
	"1.25.0":  "3.8",
	"1.24.0":  "3.7",
	"1.23.0":  "3.6",
	"1.22.0":  "3.5",
	"1.21.0":  "3.4.1",
	"default": "3.2",
}

var k8sVersionToCoreDNSTag = map[string]string{
	"1.30.0":  "1.11.1",
	"1.29.0":  "1.11.1",
	"1.28.0":  "1.10.1",
	"1.27.0":  "1.10.1",
	"1.26.0":  "1.9.3",
	"1.25.0":  "1.9.3",
	"1.24.0":  "1.8.6",
	"1.23.0":  "1.8.6",
	"1.22.0":  "1.8.4",
	"1.21.0":  "1.8.0",
	"default": "1.6.9",
}

var sortedKubeVersions = []string{
	"1.30.0", "1.29.0", "1.28.0", "1.27.0", "1.26.0", "1.25.0",
	"1.24.0", "1.23.0", "1.22.0", "1.21.0",
}

func getTagFromVersionMap(kubeVersionStr string, versionMap map[string]string) string {
	kubeVersion := versionutil.MustParseSemantic(kubeVersionStr)

	var versions []*versionutil.Version
	for k := range versionMap {
		if k != "default" {
			versions = append(versions, versionutil.MustParseSemantic(k))
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[j].LessThan(versions[i])
	})

	for _, v := range versions {
		if kubeVersion.AtLeast(v) {
			return versionMap[v.String()]
		}
	}
	return versionMap["default"]
}

func GetImageNames() []string {
	return []string{
		"pause",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-scheduler",
		"kube-proxy",
		"conformance",
		"coredns",
		"k8s-dns-node-cache",
		"calico-kube-controllers",
		"calico-cni",
		"calico-node",
		"calico-flexvol",
		"calico-typha",
		"flannel",
		"flannel-cni-plugin",
		"cilium",
		"cilium-operator-generic",
		"hybridnet",
		"kubeovn",
		"multus",
		"provisioner-localpv",
		"linux-utils",
		"haproxy",
		"nginx",
		"kubevip",
		"kata-deploy",
		"node-feature-discovery",
		"nfs-plugin",
		"csi-provisioner",
		"csi-node-driver-registrar",
		"csi-resizer",
		"csi-snapshotter",
		"tigera-operator",
		"calico-apiserver",
		"calico-kube-controllers",
		"calico-envoy-gateway",
		"calico-envoy-proxy",
		"calico-envoy-ratelimit",
		"calico-dikastes",
		"calico-pod2daemon-flexvol",
		"calico-key-cert-provisioner",
		"calico-goldmane",
		"calico-whisker",
		"calico-whisker-backend",
	}
}

func GetImages(ctx runtime.Context) []Image {
	i := Images{}
	i.Images = []Image{
		GetImage(ctx, "etcd"),
		GetImage(ctx, "pause"),
		GetImage(ctx, "kube-apiserver"),
		GetImage(ctx, "kube-controller-manager"),
		GetImage(ctx, "kube-scheduler"),
		GetImage(ctx, "kube-proxy"),
		GetImage(ctx, "coredns"),
		GetImage(ctx, "k8s-dns-node-cache"),
		GetImage(ctx, "calico-kube-controllers"),
		GetImage(ctx, "calico-cni"),
		GetImage(ctx, "calico-node"),
		GetImage(ctx, "calico-flexvol"),
		GetImage(ctx, "calico-typha"),
		GetImage(ctx, "cilium"),
		GetImage(ctx, "cilium-operator-generic"),
		GetImage(ctx, "flannel"),
		GetImage(ctx, "flannel-cni-plugin"),
		GetImage(ctx, "kubeovn"),
		GetImage(ctx, "haproxy"),
		GetImage(ctx, "kubevip"),
		GetImage(ctx, "nginx"),
		GetImage(ctx, "nfs-plugin"),
		GetImage(ctx, "csi-provisioner"),
		GetImage(ctx, "csi-node-driver-registrar"),
		GetImage(ctx, "csi-resizer"),
		GetImage(ctx, "csi-snapshotter"),
		GetImage(ctx, "tigera-operator"),
		GetImage(ctx, "calico-apiserver"),
		GetImage(ctx, "calico-kube-controllers"),
		GetImage(ctx, "calico-envoy-gateway"),
		GetImage(ctx, "calico-envoy-proxy"),
		GetImage(ctx, "calico-envoy-ratelimit"),
		GetImage(ctx, "calico-dikastes"),
		GetImage(ctx, "calico-pod2daemon-flexvol"),
		GetImage(ctx, "calico-key-cert-provisioner"),
		GetImage(ctx, "calico-goldmane"),
		GetImage(ctx, "calico-whisker"),
		GetImage(ctx, "calico-whisker-backend"),
	}
	return i.Images
}

func GetImage(context runtime.Context, name string) Image {
	kubeVersionStr := context.ClusterConfig.Spec.Kubernetes.Version
	currentKubeVersion := versionutil.MustParseSemantic(kubeVersionStr)

	pauseTag := getTagFromVersionMap(kubeVersionStr, k8sVersionToPauseTag)
	corednsTag := getTagFromVersionMap(kubeVersionStr, k8sVersionToCoreDNSTag)
	v1_21_0 := versionutil.MustParseSemantic("v1.21.0")
	isV1_21_0 := !currentKubeVersion.LessThan(v1_21_0) && !v1_21_0.LessThan(currentKubeVersion)
	if isV1_21_0 &&
		context.ClusterConfig.Spec.Kubernetes.ContainerRuntime.Type != "" &&
		context.ClusterConfig.Spec.Kubernetes.ContainerRuntime.Type != "docker" {
		pauseTag = "3.4.1"
	}
	logger.Debug("pauseTag: %s, corednsTag: %s", pauseTag, corednsTag)
	cfg := context.GetClusterConfig().Spec
	privateRegistry := cfg.Registry.MirroringAndRewriting.PrivateRegistry

	logger.Debug("pauseTag: %s, corednsTag: %s", pauseTag, corednsTag)

	ImageList := map[string]Image{
		"pause":                     {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "pause", Tag: pauseTag, Group: common.RoleKubernetes, Enable: true},
		"etcd":                      {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "etcd", Tag: common.DefaultEtcdVersion, Group: common.RoleMaster, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Etcd.Type, string(common.EtcdDeploymentTypeKubeadm))},
		"kube-apiserver":            {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-apiserver", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleRegistry, Enable: true},
		"kube-controller-manager":   {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-controller-manager", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleMaster, Enable: true},
		"kube-scheduler":            {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-scheduler", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleMaster, Enable: true},
		"kube-proxy":                {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-proxy", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleKubernetes, Enable: *context.GetClusterConfig().Spec.Kubernetes.KubeProxy.Enable},
		"coredns":                   {RepoAddr: privateRegistry, Namespace: "coredns", Repo: "coredns", Tag: corednsTag, Group: common.RoleKubernetes, Enable: true},
		"k8s-dns-node-cache":        {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "k8s-dns-node-cache", Tag: "1.22.20", Group: common.RoleKubernetes, Enable: *context.GetClusterConfig().Spec.DNS.NodeLocalDNS.Enabled},
		"calico-kube-controllers":   {RepoAddr: privateRegistry, Namespace: "calico", Repo: "kube-controllers", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-cni":                {RepoAddr: privateRegistry, Namespace: "calico", Repo: "cni", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-node":               {RepoAddr: privateRegistry, Namespace: "calico", Repo: "node", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-flexvol":            {RepoAddr: privateRegistry, Namespace: "calico", Repo: "pod2daemon-flexvol", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-typha":              {RepoAddr: privateRegistry, Namespace: "calico", Repo: "typha", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico") && len(context.GetHostsByRole(common.RoleKubernetes)) > 50},
		"flannel":                   {RepoAddr: privateRegistry, Namespace: "flannel", Repo: "flannel", Tag: common.DefaultFlannelVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "flannel")},
		"flannel-cni-plugin":        {RepoAddr: privateRegistry, Namespace: "flannel", Repo: "flannel-cni-plugin", Tag: common.DefaultCNIPluginsVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "flannel")},
		"cilium":                    {RepoAddr: privateRegistry, Namespace: "cilium", Repo: "cilium", Tag: common.DefaultCiliumVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "cilium")},
		"cilium-operator-generic":   {RepoAddr: privateRegistry, Namespace: "cilium", Repo: "operator-generic", Tag: common.DefaultCiliumVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "cilium")},
		"hybridnet":                 {RepoAddr: privateRegistry, Namespace: "hybridnetdev", Repo: "hybridnet", Tag: common.DefaulthybridnetVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "hybridnet")},
		"kubeovn":                   {RepoAddr: privateRegistry, Namespace: "kubeovn", Repo: "kube-ovn", Tag: common.DefaultKubeovnVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "kubeovn")},
		"multus":                    {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "multus-cni", Tag: common.DefalutMultusVersion, Group: common.RoleKubernetes, Enable: strings.Contains(context.GetClusterConfig().Spec.Network.Plugin, "multus")},
		"provisioner-localpv":       {RepoAddr: privateRegistry, Namespace: "openebs", Repo: "provisioner-localpv", Tag: "3.3.0", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.OpenEBS.Enabled && *context.GetClusterConfig().Spec.Storage.OpenEBS.Engines.LocalHostpath.Enabled},
		"linux-utils":               {RepoAddr: privateRegistry, Namespace: "openebs", Repo: "linux-utils", Tag: "3.3.0", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.OpenEBS.Enabled && *context.GetClusterConfig().Spec.Storage.OpenEBS.Engines.LocalHostpath.Enabled},
		"haproxy":                   {RepoAddr: privateRegistry, Namespace: "library", Repo: "haproxy", Tag: "2.9.6-alpine", Group: common.RoleWorker, Enable: context.GetClusterConfig().Spec.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeHAProxy},
		"nginx":                     {RepoAddr: privateRegistry, Namespace: "library", Repo: "nginx", Tag: "2.9.6-alpine", Group: common.RoleWorker, Enable: context.GetClusterConfig().Spec.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeNginx},
		"kubevip":                   {RepoAddr: privateRegistry, Namespace: "plndr", Repo: "kube-vip", Tag: "v0.7.2", Group: common.RoleMaster, Enable: context.GetClusterConfig().Spec.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeKubeVIP || context.GetClusterConfig().Spec.ControlPlaneEndpoint.ExternalLoadBalancerType == common.ExternalLBTypeKubeVIP},
		"kata-deploy":               {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kata-deploy", Tag: "stable", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Kubernetes.Addons.Kata.Enabled},
		"node-feature-discovery":    {RepoAddr: privateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "node-feature-discovery", Tag: "v0.10.0", Group: common.RoleKubernetes, Enable: *context.GetClusterConfig().Spec.Kubernetes.Addons.NodeFeatureDiscovery.Enabled},
		"nfs-plugin":                {RepoAddr: privateRegistry, Namespace: "sig-storage", Repo: "nfs-plugin", Tag: "3.3.0", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.NFS.Enabled},
		"csi-provisioner":           {RepoAddr: privateRegistry, Namespace: "sig-storage", Repo: "csi-provisioner", Tag: "v4.6.0", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.NFS.Enabled},
		"csi-node-driver-registrar": {RepoAddr: privateRegistry, Namespace: "sig-storage", Repo: "csi-node-driver-registrar", Tag: "v2.10.0", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.NFS.Enabled},
		"csi-resizer":               {RepoAddr: privateRegistry, Namespace: "sig-storage", Repo: "csi-resizer", Tag: "v1.10.0", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.NFS.Enabled},
		"csi-snapshotter":           {RepoAddr: privateRegistry, Namespace: "sig-storage", Repo: "csi-snapshotter", Tag: "v7.0.1", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Storage.NFS.Enabled},
		"tigera-operator":           {RepoAddr: privateRegistry, Namespace: "tigera", Repo: "operator", Tag: "v1.38.3", Group: common.RoleKubernetes, Enable: context.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCalico)},
	}

	image := ImageList[name]
	if context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceOverride != "" {
		image.NamespaceOverride = context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceOverride
	} else if context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceRewrite != nil {
		image.NamespaceRewrite = context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceRewrite
	}
	return image
}
