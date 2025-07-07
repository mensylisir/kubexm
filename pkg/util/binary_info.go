// Package util provides utility functions for the kubexm project.
package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
)

// BinaryType represents the type of binary component.
type BinaryType string

// Constants for BinaryType
const (
	ETCD       BinaryType = "etcd"
	KUBE       BinaryType = "kubernetes" // For kubeadm, kubelet, kubectl, etc.
	CNI        BinaryType = "cni"
	HELM       BinaryType = "helm"
	DOCKER     BinaryType = "docker"
	CRIDOCKERD BinaryType = "cri-dockerd"
	CRICTL     BinaryType = "crictl"
	K3S        BinaryType = "k3s"        // K3S is a KUBE type distribution
	K8E        BinaryType = "k8e"        // K8E is a KUBE type distribution
	REGISTRY   BinaryType = "registry"   // For local registry components like 'registry' or 'harbor'
	BUILD      BinaryType = "build"      // For build tools like 'buildx'
	CONTAINERD BinaryType = "containerd" // For containerd itself
	RUNC       BinaryType = "runc"       // For runc
	CALICOCTL  BinaryType = "calicoctl"  // For calicoctl CLI
	UNKNOWN    BinaryType = "unknown"    // Fallback type
)

// Constants for component names, matching keys in knownBinaryDetails
const (
	ComponentEtcd                  = "etcd"
	ComponentKubeadm               = "kubeadm"
	ComponentKubelet               = "kubelet"
	ComponentKubectl               = "kubectl"
	ComponentKubeProxy             = "kube-proxy"
	ComponentKubeScheduler         = "kube-scheduler"
	ComponentKubeControllerManager = "kube-controller-manager"
	ComponentKubeApiServer         = "kube-apiserver"
	ComponentKubeCNI               = "kubecni" // Name for CNI plugins bundle
	ComponentHelm                  = "helm"
	ComponentDocker                = "docker"
	ComponentCriDockerd            = "cri-dockerd"
	ComponentCriCtl                = "crictl"
	ComponentK3s                   = "k3s"
	ComponentK8e                   = "k8e"
	ComponentRegistry              = "registry" // Generic name for the registry binary itself
	ComponentHarbor                = "harbor"
	ComponentCompose               = "compose" // Docker Compose
	ComponentContainerd            = "containerd"
	ComponentRunc                  = "runc"
	ComponentCalicoCtl             = "calicoctl"
	ComponentBuildx                = "buildx"
)

// Directory name constants under CLUSTER_NAME path
const (
	DirNameCerts            = "certs"
	DirNameEtcd             = common.DefaultEtcdDir             // "etcd"
	DirNameContainerRuntime = common.DefaultContainerRuntimeDir // "container_runtime"
	DirNameKubernetes       = common.DefaultKubernetesDir       // "kubernetes"
)

// BinaryInfo holds information about a downloadable binary component.
type BinaryInfo struct {
	Component    string     // User-friendly name, e.g., "etcd", "kubeadm"
	Type         BinaryType // Type of the component
	Version      string
	Arch         string
	OS           string // Operating system, e.g., "linux"
	Zone         string // Download zone, e.g., "cn" or "" for default
	FileName     string // Filename of the download, e.g., "etcd-v3.5.9-linux-amd64.tar.gz"
	URL          string // Download URL
	IsArchive            bool   // True if the downloaded file is an archive (.tar.gz, .tgz, .zip)
	BaseDir              string // Base directory for storing this binary type locally: ${WORK_DIR}/.kubexm/${CLUSTER_NAME}/${TypeDirName}/
	ComponentDir         string // Specific directory for this component: ${BaseDir}/${ComponentSubDir}/${Version}/${Arch}/ or ${BaseDir}/${Version}/${Arch}/
	FilePath             string // Full local path to the downloaded file: ${ComponentDir}/${FileName}
	ExpectedChecksum     string // Expected checksum value (e.g., the hex string)
	ExpectedChecksumType string // Type of the checksum (e.g., "sha256", "sha512")
}

// BinaryDetailSpec holds the static configuration for a known binary component.
type BinaryDetailSpec struct {
	BinaryType           BinaryType
	URLTemplate          string
	CNURLTemplate        string
	FileNameTemplate     string
	IsArchive            bool
	DefaultOS            string
	ComponentNameForDir  string // Used for container_runtime subdirectories like "docker", "containerd"
	ExpectedChecksum     string // Optional: Checksum string (e.g., "abc...def")
	ExpectedChecksumType string // Optional: Checksum type (e.g., "sha256")
}

// BinaryProvider holds the configuration for known binaries and provides methods to get their info.
type BinaryProvider struct {
	details map[string]BinaryDetailSpec
	// Potentially add fields for custom config paths or overrides here in the future
}

// NewBinaryProvider creates a new BinaryProvider with default known binary details.
func NewBinaryProvider() *BinaryProvider {
	return &BinaryProvider{
		details: defaultKnownBinaryDetails, // Use a new var for default map
	}
}

// defaultKnownBinaryDetails is the internal default map of component names to their download attributes.
// This will be assigned to BinaryProvider.details by default.
var defaultKnownBinaryDetails = map[string]BinaryDetailSpec{
	// Corrected field names to match BinaryDetailSpec struct
	ComponentEtcd: {
		BinaryType:           ETCD,
		URLTemplate:          "https://github.com/coreos/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		CNURLTemplate:        "https://kubernetes-release.pek3b.qingstor.com/etcd/release/download/{{.Version}}/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		FileNameTemplate:     "etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		IsArchive:            true,
		DefaultOS:            "linux",
		ExpectedChecksum:     "dummy-etcd-checksum-val",
		ExpectedChecksumType: "sha256",
	},
	ComponentKubeadm: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubeadm",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubeadm",
		FileNameTemplate: "kubeadm",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubelet: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubelet",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubelet",
		FileNameTemplate: "kubelet",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubectl: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubectl",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubectl",
		FileNameTemplate: "kubectl",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubeProxy: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-proxy",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-proxy",
		FileNameTemplate: "kube-proxy",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubeScheduler: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-scheduler",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-scheduler",
		FileNameTemplate: "kube-scheduler",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubeControllerManager: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-controller-manager",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-controller-manager",
		FileNameTemplate: "kube-controller-manager",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubeApiServer: {
		BinaryType:       KUBE,
		URLTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-apiserver",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-apiserver",
		FileNameTemplate: "kube-apiserver",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentKubeCNI: {
		BinaryType:       CNI,
		URLTemplate:      "https://github.com/containernetworking/plugins/releases/download/{{.Version}}/cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		CNURLTemplate:    "https://containernetworking.pek3b.qingstor.com/plugins/releases/download/{{.Version}}/cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		FileNameTemplate: "cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		IsArchive:        true,
		DefaultOS:        "linux",
	},
	ComponentHelm: {
		BinaryType:       HELM,
		URLTemplate:      "https://get.helm.sh/helm-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-helm.pek3b.qingstor.com/linux-{{.Arch}}/{{.Version}}/helm-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		FileNameTemplate: "helm-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		IsArchive:        true,
		DefaultOS:        "linux",
	},
	ComponentDocker: {
		BinaryType:          DOCKER,
		URLTemplate:         "https://download.docker.com/linux/static/stable/{{.ArchAlias}}/docker-{{.VersionNoV}}.tgz",
		CNURLTemplate:       "https://mirrors.aliyun.com/docker-ce/linux/static/stable/{{.ArchAlias}}/docker-{{.VersionNoV}}.tgz",
		FileNameTemplate:    "docker-{{.VersionNoV}}.tgz",
		IsArchive:           true,
		DefaultOS:           "linux",
		ComponentNameForDir: "docker",
	},
	ComponentCriDockerd: {
		BinaryType:          CRIDOCKERD,
		URLTemplate:         "https://github.com/Mirantis/cri-dockerd/releases/download/v{{.VersionNoV}}/cri-dockerd-{{.VersionNoV}}.{{.Arch}}.tgz",
		CNURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/cri-dockerd/releases/download/v{{.VersionNoV}}/cri-dockerd-{{.VersionNoV}}.{{.Arch}}.tgz",
		FileNameTemplate:    "cri-dockerd-{{.VersionNoV}}.{{.Arch}}.tgz",
		IsArchive:           true,
		DefaultOS:           "linux",
		ComponentNameForDir: "cri-dockerd",
	},
	ComponentCriCtl: {
		BinaryType:       CRICTL,
		URLTemplate:      "https://github.com/kubernetes-sigs/cri-tools/releases/download/{{.Version}}/crictl-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/cri-tools/releases/download/{{.Version}}/crictl-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		FileNameTemplate: "crictl-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		IsArchive:        true,
		DefaultOS:        "linux",
	},
	ComponentK3s: {
		BinaryType:       K3S,
		URLTemplate:      "https://github.com/k3s-io/k3s/releases/download/{{.VersionWithPlus}}/k3s{{.ArchSuffix}}",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/k3s/releases/download/{{.VersionWithPlus}}/linux/{{.Arch}}/k3s",
		FileNameTemplate: "k3s{{.ArchSuffix}}",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentK8e: {
		BinaryType:       K8E,
		URLTemplate:      "https://github.com/xiaods/k8e/releases/download/{{.VersionWithPlus}}/k8e{{.ArchSuffix}}",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/k8e/releases/download/{{.VersionWithPlus}}/linux/{{.Arch}}/k8e",
		FileNameTemplate: "k8e{{.ArchSuffix}}",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentRegistry: {
		BinaryType:          REGISTRY,
		URLTemplate:         "https://github.com/kubesphere/kubekey/releases/download/v2.0.0-alpha.1/registry-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		CNURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/registry/{{.VersionNoV}}/registry-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		FileNameTemplate:    "registry-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		IsArchive:           true,
		DefaultOS:           "linux",
		ComponentNameForDir: "registry",
	},
	ComponentHarbor: {
		BinaryType:          REGISTRY,
		URLTemplate:         "https://github.com/goharbor/harbor/releases/download/{{.Version}}/harbor-offline-installer-{{.Version}}.tgz",
		CNURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/harbor/releases/download/{{.Version}}/harbor-offline-installer-{{.Version}}.tgz",
		FileNameTemplate:    "harbor-offline-installer-{{.Version}}.tgz",
		IsArchive:           true,
		DefaultOS:           "linux",
		ComponentNameForDir: "harbor",
	},
	ComponentCompose: {
		BinaryType:          REGISTRY,
		URLTemplate:         "https://github.com/docker/compose/releases/download/{{.Version}}/docker-compose-{{.OS}}-{{.ArchAlias}}",
		CNURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/docker/compose/releases/download/{{.Version}}/docker-compose-{{.OS}}-{{.ArchAlias}}",
		FileNameTemplate:    "docker-compose-{{.OS}}-{{.ArchAlias}}",
		IsArchive:           false,
		DefaultOS:           "linux",
		ComponentNameForDir: "compose",
	},
	ComponentContainerd: {
		BinaryType:          CONTAINERD,
		URLTemplate:         "https://github.com/containerd/containerd/releases/download/v{{.VersionNoV}}/containerd-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		CNURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/containerd/containerd/releases/download/v{{.VersionNoV}}/containerd-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		FileNameTemplate:    "containerd-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		IsArchive:           true,
		DefaultOS:           "linux",
		ComponentNameForDir: "containerd",
	},
	ComponentRunc: {
		BinaryType:          RUNC,
		URLTemplate:         "https://github.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		CNURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		FileNameTemplate:    "runc.{{.Arch}}",
		IsArchive:           false,
		DefaultOS:           "linux",
		ComponentNameForDir: "runc",
	},
	ComponentCalicoCtl: {
		BinaryType:       CALICOCTL,
		URLTemplate:      "https://github.com/projectcalico/calico/releases/download/{{.Version}}/calicoctl-{{.OS}}-{{.Arch}}",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/projectcalico/calico/releases/download/{{.Version}}/calicoctl-{{.OS}}-{{.Arch}}",
		FileNameTemplate: "calicoctl-{{.OS}}-{{.Arch}}",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
	ComponentBuildx: {
		BinaryType:       BUILD,
		URLTemplate:      "https://github.com/docker/buildx/releases/download/{{.Version}}/buildx-{{.Version}}.{{.OS}}-{{.Arch}}",
		CNURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/docker/buildx/releases/download/{{.Version}}/buildx-{{.Version}}.{{.OS}}-{{.Arch}}",
		FileNameTemplate: "buildx-{{.Version}}.{{.OS}}-{{.Arch}}",
		IsArchive:        false,
		DefaultOS:        "linux",
	},
}

// templateData is used for rendering URL and filename templates.
type templateData struct {
	Version         string // Full version string, e.g., "v1.2.3", "v1.2.3+k3s1"
	VersionNoV      string // Version without 'v' prefix, e.g., "1.2.3", "1.2.3+k3s1"
	VersionWithPlus string // Original version if it contains '+', used for k3s/k8e that include it directly in some URL parts
	Arch            string
	ArchAlias       string // e.g., x86_64 for amd64
	ArchSuffix      string // e.g., -arm64 for k3s arm64, empty otherwise
	OS              string
}

// GetBinaryInfo returns information about a downloadable binary component.
// componentName should be one of the Component* constants.
// version should be like "v1.2.3" (for most) or "1.2.3" (for cri-dockerd, docker) or "v1.2.3+k3s1" (for k3s).
// arch should be "amd64" or "arm64". If empty, defaults to runtime.GOARCH.
// zone is "cn" for China region, otherwise uses default URLs.
// workDir and clusterName are used to construct local storage paths.
func (bp *BinaryProvider) GetBinaryInfo(componentName, version, arch, zone, workDir, clusterName string) (*BinaryInfo, error) {
	if strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("version cannot be empty for component %s", componentName)
	}
	details, ok := bp.details[strings.ToLower(componentName)]
	if !ok {
		return nil, fmt.Errorf("unknown binary component: %s", componentName)
	}

	finalArch := arch
	if finalArch == "" {
		finalArch = runtime.GOARCH // Default to host architecture
	}
	// Normalize architecture names
	switch finalArch {
	case "x86_64":
		finalArch = "amd64"
	case "aarch64":
		finalArch = "arm64"
	}

	finalOS := details.DefaultOS // Corrected
	if finalOS == "" {
		finalOS = "linux" // Ultimate fallback
	}

	// Prepare versions for template
	versionNoV := strings.TrimPrefix(version, "v")
	versionWithPlus := version // Default to original version string

	// Specific handling for k3s/k8e naming
	archSuffix := ""
	if componentName == ComponentK3s || componentName == ComponentK8e {
		if finalArch == "arm64" {
			archSuffix = "-" + finalArch
		}
		// k3s/k8e versions in URLs often look like "v1.20.0+k3s1"
		// The `version` param should already be in this format if needed.
	}

	td := templateData{
		Version:         version,
		VersionNoV:      versionNoV,
		VersionWithPlus: versionWithPlus,
		Arch:            finalArch,
		ArchAlias:       ArchAlias(finalArch),
		ArchSuffix:      archSuffix,
		OS:              finalOS,
	}

	urlTmplToUse := details.URLTemplate // Corrected
	if strings.ToLower(zone) == "cn" && details.CNURLTemplate != "" { // Corrected
		urlTmplToUse = details.CNURLTemplate // Corrected
	}

	fileName, err := RenderTemplate(details.FileNameTemplate, td) // Corrected
	if err != nil {
		return nil, fmt.Errorf("failed to render filename for %s (template: '%s'): %w", componentName, details.FileNameTemplate, err) // Corrected
	}

	downloadURL, err := RenderTemplate(urlTmplToUse, td)
	if err != nil {
		return nil, fmt.Errorf("failed to render download URL for %s (template: '%s'): %w", componentName, urlTmplToUse, err)
	}

	// Path construction based on 21-其他说明.md and 22-额外要求.md
	// workdir/.kubexm/${cluster_name}/${type_dir_name}/${sub_path_parts...}
	if workDir == "" {
		return nil, fmt.Errorf("workDir cannot be empty for generating binary path")
	}
	if clusterName == "" {
		return nil, fmt.Errorf("clusterName cannot be empty for generating binary path")
	}

	kubexmRoot := filepath.Join(workDir, common.KubexmRootDirName) // Updated to KubexmRootDirName
	clusterBaseDir := filepath.Join(kubexmRoot, clusterName)

	var typeSpecificBaseDir string
	var componentVersionSpecificDir string

	switch details.BinaryType { // Corrected
	case ETCD:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameEtcd)
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, version, finalArch)
	case KUBE, K3S, K8E: // Group all Kubernetes-like distributions here for pathing
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameKubernetes)
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, version, finalArch)
	case CNI, CALICOCTL: // CNI plugins and related tools like calicoctl
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameKubernetes, "cni") // Store CNI under kubernetes/cni
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, componentName, version, finalArch)
	case CONTAINERD, DOCKER, RUNC, CRIDOCKERD, CRICTL:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameContainerRuntime)
		compDirName := details.ComponentNameForDir // Corrected
		if compDirName == "" {
			compDirName = componentName
		}
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, compDirName, version, finalArch)
	case HELM, BUILD, REGISTRY: // Tools like Helm, buildx, registry, harbor, compose
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, string(details.BinaryType)) // Corrected
		compDirName := details.ComponentNameForDir // Corrected
		if compDirName == "" {
			compDirName = componentName
		}
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, compDirName, version, finalArch)

	default:
		return nil, fmt.Errorf("unhandled binary type '%s' for path construction", details.BinaryType) // Corrected
	}

	filePath := filepath.Join(componentVersionSpecificDir, fileName)

	return &BinaryInfo{
		Component:    componentName,
		Type:         details.BinaryType, // Corrected
		Version:      version,
		Arch:         finalArch,
		OS:           finalOS,
		Zone:         zone,
		FileName:     fileName,
		URL:          downloadURL,
		IsArchive:    details.IsArchive, // Corrected
		BaseDir:              typeSpecificBaseDir,
		ComponentDir:         componentVersionSpecificDir,
		FilePath:             filePath,
		ExpectedChecksum:     details.ExpectedChecksum,     // Corrected
		ExpectedChecksumType: details.ExpectedChecksumType, // Corrected
	}, nil
}

// GetZone retrieves the download zone from the KXZONE environment variable.
func GetZone() string {
	if strings.ToLower(os.Getenv("KXZONE")) == "cn" {
		return "cn"
	}
	return ""
}

// ArchAlias returns the architecture alias typically used in download URLs.
func ArchAlias(arch string) string {
	switch strings.ToLower(arch) {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return arch
	}
}

// GetImageNames returns a predefined list of core Kubernetes image names.
// This list is based on the markdown file "23-二进制下载地址.md".
func GetImageNames() []string {
	return []string{
		"pause",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-scheduler",
		"kube-proxy",
		"conformance:v1.33.0", // Example version, actual version might vary
		// Network
		"coredns", // Usually coredns/coredns
		"k8s-dns-node-cache",
		"calico-kube-controllers",
		"calico-cni",
		"calico-node",
		"calico-flexvol",
		"calico-typha",
		"flannel", // Usually flannelcni/flannel or quay.io/coreos/flannel
		"flannel-cni-plugin",
		"cilium", // Usually cilium/cilium
		"cilium-operator-generic",
		"hybridnet",
		"kubeovn",
		"multus", // Usually nfvpe/multus-cni
		// Storage
		"provisioner-localpv", // e.g., sig-storage/local-volume-provisioner
		"linux-utils",         // Often used by storage plugins for formatting etc.
		// Load Balancer
		"haproxy",
		"nginx",
		"kubevip", // Usually an image like anthonytatowicz/kubevip
		// Kata-deploy
		"kata-deploy",
		// Node-feature-discovery
		"node-feature-discovery", // e.g., k8s.gcr.io/nfd/node-feature-discovery
	}
}
