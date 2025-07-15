package images

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"strings"
)

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
		GetImage(ctx, "cilium"),
		GetImage(ctx, "cilium-operator-generic"),
		GetImage(ctx, "flannel"),
		GetImage(ctx, "flannel-cni-plugin"),
		GetImage(ctx, "kubeovn"),
		GetImage(ctx, "haproxy"),
		GetImage(ctx, "kubevip"),
	}
	return i.Images
}

func GetImage(context runtime.Context, name string) Image {
	var image Image
	pauseTag, corednsTag := "3.2", "1.6.9"

	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).LessThan(versionutil.MustParseSemantic("v1.21.0")) {
		pauseTag = "3.2"
		corednsTag = "1.6.9"
	}
	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).AtLeast(versionutil.MustParseSemantic("v1.21.0")) ||
		(context.ClusterConfig.Spec.Kubernetes.ContainerRuntime.Type != "" && context.ClusterConfig.Spec.Kubernetes.ContainerRuntime.Type != "docker") {
		pauseTag = "3.4.1"
		corednsTag = "1.8.0"
	}
	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).AtLeast(versionutil.MustParseSemantic("v1.22.0")) {
		pauseTag = "3.5"
		corednsTag = "1.8.0"
	}
	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).AtLeast(versionutil.MustParseSemantic("v1.23.0")) {
		pauseTag = "3.6"
		corednsTag = "1.8.6"
	}
	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).AtLeast(versionutil.MustParseSemantic("v1.24.0")) {
		pauseTag = "3.7"
		corednsTag = "1.8.6"
	}
	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).AtLeast(versionutil.MustParseSemantic("v1.25.0")) {
		pauseTag = "3.8"
		corednsTag = "1.9.3"
	}
	if versionutil.MustParseSemantic(context.ClusterConfig.Spec.Kubernetes.Version).AtLeast(versionutil.MustParseSemantic("v1.26.0")) {
		pauseTag = "3.9"
		corednsTag = "1.9.3"
	}

	logger.Debug("pauseTag: %s, corednsTag: %s", pauseTag, corednsTag)

	ImageList := map[string]Image{
		"pause":                   {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "pause", Tag: pauseTag, Group: common.RoleKubernetes, Enable: true},
		"etcd":                    {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "etcd", Tag: common.DefaultEtcdVersion, Group: common.RoleMaster, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Etcd.Type, string(common.EtcdDeploymentTypeKubeadm))},
		"kube-apiserver":          {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-apiserver", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleRegistry, Enable: true},
		"kube-controller-manager": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-controller-manager", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleMaster, Enable: true},
		"kube-scheduler":          {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-scheduler", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleMaster, Enable: true},
		"kube-proxy":              {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kube-proxy", Tag: context.GetClusterConfig().Spec.Kubernetes.Version, Group: common.RoleKubernetes, Enable: *context.GetClusterConfig().Spec.Kubernetes.KubeProxy.Enable},

		// network
		"coredns":                 {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "coredns", Repo: "coredns", Tag: corednsTag, Group: common.RoleKubernetes, Enable: true},
		"k8s-dns-node-cache":      {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "k8s-dns-node-cache", Tag: "1.22.20", Group: common.RoleKubernetes, Enable: *context.GetClusterConfig().Spec.DNS.NodeLocalDNS.Enabled},
		"calico-kube-controllers": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "calico", Repo: "kube-controllers", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-cni":              {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "calico", Repo: "cni", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-node":             {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "calico", Repo: "node", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-flexvol":          {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "calico", Repo: "pod2daemon-flexvol", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico")},
		"calico-typha":            {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "calico", Repo: "typha", Tag: common.DefaultCalicoVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "calico") && len(context.GetHostsByRole(common.RoleKubernetes)) > 50},
		"flannel":                 {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "flannel", Repo: "flannel", Tag: common.DefaultFlannelVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "flannel")},
		"flannel-cni-plugin":      {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "flannel", Repo: "flannel-cni-plugin", Tag: common.DefaultCNIPluginsVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "flannel")},
		"cilium":                  {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "cilium", Repo: "cilium", Tag: common.DefaultCiliumVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "cilium")},
		"cilium-operator-generic": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "cilium", Repo: "operator-generic", Tag: common.DefaultCiliumVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "cilium")},
		"hybridnet":               {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "hybridnetdev", Repo: "hybridnet", Tag: common.DefaulthybridnetVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "hybridnet")},
		"kubeovn":                 {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "kubeovn", Repo: "kube-ovn", Tag: common.DefaultKubeovnVersion, Group: common.RoleKubernetes, Enable: strings.EqualFold(context.GetClusterConfig().Spec.Network.Plugin, "kubeovn")},
		"multus":                  {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "multus-cni", Tag: common.DefalutMultusVersion, Group: common.RoleKubernetes, Enable: strings.Contains(context.GetClusterConfig().Spec.Network.Plugin, "multus")},
		// storage
		"provisioner-localpv": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "openebs", Repo: "provisioner-localpv", Tag: "3.3.0", Group: common.RoleWorker, Enable: false},
		"linux-utils":         {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "openebs", Repo: "linux-utils", Tag: "3.3.0", Group: common.RoleWorker, Enable: false},
		// load balancer
		"haproxy": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "library", Repo: "haproxy", Tag: "2.9.6-alpine", Group: common.RoleWorker, Enable: context.GetClusterConfig().Spec.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeHAProxy},
		"nginx":   {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "library", Repo: "nginx", Tag: "2.9.6-alpine", Group: common.RoleWorker, Enable: context.GetClusterConfig().Spec.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeNginx},
		"kubevip": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "plndr", Repo: "kube-vip", Tag: "v0.7.2", Group: common.RoleMaster, Enable: context.GetClusterConfig().Spec.ControlPlaneEndpoint.InternalLoadBalancerType == common.InternalLBTypeKubeVIP},
		// kata-deploy
		"kata-deploy": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "kata-deploy", Tag: "stable", Group: common.RoleWorker, Enable: *context.GetClusterConfig().Spec.Kubernetes.Addons.Kata.Enabled},
		// node-feature-discovery
		"node-feature-discovery":    {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: common.DefaultKubeImageNamespace, Repo: "node-feature-discovery", Tag: "v0.10.0", Group: common.RoleKubernetes, Enable: *context.GetClusterConfig().Spec.Kubernetes.Addons.NodeFeatureDiscovery.Enabled},
		"nfs-plugin":                {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "sig-storage", Repo: "nfs-plugin", Tag: "3.3.0", Group: common.RoleWorker, Enable: false},
		"csi-provisioner":           {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "sig-storage", Repo: "csi-provisioner", Tag: "v4.6.0", Group: common.RoleWorker, Enable: false},
		"csi-node-driver-registrar": {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "sig-storage", Repo: "csi-node-driver-registrar", Tag: "v2.10.0", Group: common.RoleWorker, Enable: false},
		"csi-resizer":               {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "sig-storage", Repo: "csi-resizer", Tag: "v1.10.0", Group: common.RoleWorker, Enable: false},
		"csi-snapshotter":           {RepoAddr: context.ClusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry, Namespace: "sig-storage", Repo: "csi-snapshotter", Tag: "v7.0.1", Group: common.RoleWorker, Enable: false},
	}

	image = ImageList[name]
	if context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceOverride != "" {
		image.NamespaceOverride = context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceOverride
	}
	if context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceRewrite != nil {
		image.NamespaceRewrite = context.ClusterConfig.Spec.Registry.MirroringAndRewriting.NamespaceRewrite
	}
	return image
}
