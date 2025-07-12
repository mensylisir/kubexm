package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// ComponentType represents the type of binary component
type ComponentType string

const (
	ETCD       ComponentType = "etcd"
	KUBE       ComponentType = "kubernetes"
	CNI        ComponentType = "cni"
	HELM       ComponentType = "helm"
	DOCKER     ComponentType = "docker"
	CRIDOCKERD ComponentType = "cri-dockerd"
	CRICTL     ComponentType = "crictl"
	REGISTRY   ComponentType = "registry"
	BUILD      ComponentType = "build"
	CONTAINERD ComponentType = "containerd"
	RUNC       ComponentType = "runc"
)

// BinaryMetadata holds metadata for a binary component
type BinaryMetadata struct {
	Name             string
	Type             ComponentType
	FileNameTemplate string
	URLTemplate      string
	CNURLTemplate    string // China mirror URL template
	IsArchive        bool
	InternalPath     string // Path inside archive if applicable
}

// KubeBinary represents a Kubernetes binary component
type KubeBinary struct {
	ID       string
	Arch     string
	Version  string
	Zone     string
	Type     ComponentType
	FileName string
	Url      string
	BaseDir  string
}

// BinaryRepository holds all binary metadata
var BinaryRepository = map[string]BinaryMetadata{
	"etcd": {
		Name:             "etcd",
		Type:             ETCD,
		FileNameTemplate: "etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		URLTemplate:      "https://github.com/coreos/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/etcd/release/download/{{.Version}}/etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		IsArchive:        true,
		InternalPath:     "etcd-{{.Version}}-linux-{{.Arch}}/etcd",
	},
	"kubeadm": {
		Name:          "kubeadm",
		Type:          KUBE,
		FileName:      "kubeadm",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kubeadm",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kubeadm",
		IsArchive:     false,
	},
	"kubelet": {
		Name:          "kubelet",
		Type:          KUBE,
		FileName:      "kubelet",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kubelet",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kubelet",
		IsArchive:     false,
	},
	"kubectl": {
		Name:          "kubectl",
		Type:          KUBE,
		FileName:      "kubectl",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kubectl",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kubectl",
		IsArchive:     false,
	},
	"kube-proxy": {
		Name:          "kube-proxy",
		Type:          KUBE,
		FileName:      "kube-proxy",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kube-proxy",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kube-proxy",
		IsArchive:     false,
	},
	"kube-scheduler": {
		Name:          "kube-scheduler",
		Type:          KUBE,
		FileName:      "kube-scheduler",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kube-scheduler",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kube-scheduler",
		IsArchive:     false,
	},
	"kube-controller-manager": {
		Name:          "kube-controller-manager",
		Type:          KUBE,
		FileName:      "kube-controller-manager",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kube-controller-manager",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kube-controller-manager",
		IsArchive:     false,
	},
	"kube-apiserver": {
		Name:          "kube-apiserver",
		Type:          KUBE,
		FileName:      "kube-apiserver",
		URLTemplate:   "https://dl.k8s.io/release/{{.Version}}/bin/linux/{{.Arch}}/kube-apiserver",
		CNURLTemplate: "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/linux/{{.Arch}}/kube-apiserver",
		IsArchive:     false,
	},
	"kubecni": {
		Name:             "kubecni",
		Type:             CNI,
		FileNameTemplate: "cni-plugins-linux-{{.Arch}}-{{.Version}}.tgz",
		URLTemplate:      "https://github.com/containernetworking/plugins/releases/download/{{.Version}}/cni-plugins-linux-{{.Arch}}-{{.Version}}.tgz",
		CNURLTemplate:    "https://containernetworking.pek3b.qingstor.com/plugins/releases/download/{{.Version}}/cni-plugins-linux-{{.Arch}}-{{.Version}}.tgz",
		IsArchive:        true,
	},
	"helm": {
		Name:             "helm",
		Type:             HELM,
		FileNameTemplate: "helm-{{.Version}}-linux-{{.Arch}}.tar.gz",
		URLTemplate:      "https://get.helm.sh/helm-{{.Version}}-linux-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-helm.pek3b.qingstor.com/linux-{{.Arch}}/{{.Version}}/helm",
		IsArchive:        true,
	},
	"docker": {
		Name:             "docker",
		Type:             DOCKER,
		FileNameTemplate: "docker-{{.Version}}.tgz",
		URLTemplate:      "https://download.docker.com/linux/static/stable/{{.ArchAlias}}/docker-{{.Version}}.tgz",
		CNURLTemplate:    "https://mirrors.aliyun.com/docker-ce/linux/static/stable/{{.ArchAlias}}/docker-{{.Version}}.tgz",
		IsArchive:        true,
	},
	"cridockerd": {
		Name:             "cridockerd",
		Type:             CRIDOCKERD,
		FileNameTemplate: "cri-dockerd-{{.Version}}.tgz",
		URLTemplate:      "https://github.com/Mirantis/cri-dockerd/releases/download/v{{.Version}}/cri-dockerd-{{.Version}}.{{.Arch}}.tgz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/cri-dockerd/releases/download/v{{.Version}}/cri-dockerd-{{.Version}}.{{.Arch}}.tgz",
		IsArchive:        true,
	},
	"crictl": {
		Name:             "crictl",
		Type:             CRICTL,
		FileNameTemplate: "crictl-{{.Version}}-linux-{{.Arch}}.tar.gz",
		URLTemplate:      "https://github.com/kubernetes-sigs/cri-tools/releases/download/{{.Version}}/crictl-{{.Version}}-linux-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/cri-tools/releases/download/{{.Version}}/crictl-{{.Version}}-linux-{{.Arch}}.tar.gz",
		IsArchive:        true,
	},
	"containerd": {
		Name:             "containerd",
		Type:             CONTAINERD,
		FileNameTemplate: "containerd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		URLTemplate:      "https://github.com/containerd/containerd/releases/download/v{{.Version}}/containerd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/containerd/containerd/releases/download/v{{.Version}}/containerd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		IsArchive:        true,
	},
	"runc": {
		Name:             "runc",
		Type:             RUNC,
		FileNameTemplate: "runc.{{.Arch}}",
		URLTemplate:      "https://github.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		IsArchive:        false,
	},
	"registry": {
		Name:             "registry",
		Type:             REGISTRY,
		FileNameTemplate: "registry-{{.Version}}-linux-{{.Arch}}.tar.gz",
		URLTemplate:      "https://github.com/kubesphere/kubekey/releases/download/v2.0.0-alpha.1/registry-{{.Version}}-linux-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/registry/{{.Version}}/registry-{{.Version}}-linux-{{.Arch}}.tar.gz",
		IsArchive:        true,
	},
	"harbor": {
		Name:             "harbor",
		Type:             REGISTRY,
		FileNameTemplate: "harbor-offline-installer-{{.Version}}.tgz",
		URLTemplate:      "https://github.com/goharbor/harbor/releases/download/{{.Version}}/harbor-offline-installer-{{.Version}}.tgz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/harbor/releases/download/{{.Version}}/harbor-offline-installer-{{.Version}}.tgz",
		IsArchive:        true,
	},
}

// ImageRepository holds all required image names
var ImageRepository = []string{
	"pause",
	"kube-apiserver",
	"kube-controller-manager",
	"kube-scheduler",
	"kube-proxy",
	"conformance:v1.33.0",
	// network
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
	// storage
	"provisioner-localpv",
	"linux-utils",
	// load balancer
	"haproxy",
	"nginx",
	"kubevip",
	// kata-deploy
	"kata-deploy",
	// node-feature-discovery
	"node-feature-discovery",
}

// GetBinaryMetadata returns metadata for a binary component
func GetBinaryMetadata(componentName string) (BinaryMetadata, error) {
	metadata, exists := BinaryRepository[componentName]
	if !exists {
		return BinaryMetadata{}, fmt.Errorf("unsupported binary component: %s", componentName)
	}
	return metadata, nil
}

// NewKubeBinary creates a new KubeBinary instance with the given parameters
func NewKubeBinary(name, arch, version, prePath string) (*KubeBinary, error) {
	metadata, err := GetBinaryMetadata(name)
	if err != nil {
		return nil, err
	}

	component := &KubeBinary{
		ID:      name,
		Arch:    arch,
		Version: version,
		Zone:    os.Getenv("KXZONE"),
		Type:    metadata.Type,
	}

	// Generate filename
	if metadata.FileNameTemplate != "" {
		component.FileName = renderTemplate(metadata.FileNameTemplate, map[string]interface{}{
			"Version":   version,
			"Arch":      arch,
			"ArchAlias": ArchAlias(arch),
		})
	} else {
		component.FileName = metadata.FileName
	}

	// Generate URL based on zone
	urlTemplate := metadata.URLTemplate
	if component.Zone == "cn" && metadata.CNURLTemplate != "" {
		urlTemplate = metadata.CNURLTemplate
	}

	component.Url = renderTemplate(urlTemplate, map[string]interface{}{
		"Version":   version,
		"Arch":      arch,
		"ArchAlias": ArchAlias(arch),
	})

	// Set base directory
	component.BaseDir = filepath.Join(prePath, string(component.Type), component.Version, component.Arch)

	return component, nil
}

// GetAllRequiredImages returns all required images based on cluster configuration
func GetAllRequiredImages(clusterConfig interface{}) []string {
	// TODO: Implement logic to filter images based on cluster configuration
	// For now, return all images
	return ImageRepository
}

// renderTemplate renders a simple template with given data
func renderTemplate(template string, data map[string]interface{}) string {
	result := template
	for key, value := range data {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = fmt.Sprintf(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// ArchAlias returns architecture alias for specific components
func ArchAlias(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return arch
	}
}