// Package util provides utility functions for the kubexm project.
package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BinaryType represents the type of binary component.
type BinaryType string

const (
	ETCD        BinaryType = "etcd"
	KUBE        BinaryType = "kubernetes" // For kubeadm, kubelet, kubectl, etc.
	CNI         BinaryType = "cni"
	HELM        BinaryType = "helm"
	DOCKER      BinaryType = "docker"
	CRIDOCKERD  BinaryType = "cri-dockerd"
	CRICTL      BinaryType = "crictl"
	K3S         BinaryType = "k3s"        // K3S is a KUBE type distribution
	K8E         BinaryType = "k8e"        // K8E is a KUBE type distribution
	REGISTRY    BinaryType = "registry"   // For local registry components like 'registry' or 'harbor'
	BUILD       BinaryType = "build"      // For build tools like 'buildx'
	CONTAINERD  BinaryType = "containerd" // For containerd itself
	RUNC        BinaryType = "runc"       // For runc
	CALICOCTL   BinaryType = "calicoctl"  // For calicoctl CLI
	UNKNOWN     BinaryType = "unknown"
)

// BinaryInfo holds information about a downloadable binary component.
type BinaryInfo struct {
	Component    string     // User-friendly name, e.g., "etcd", "kubeadm"
	Type         BinaryType // Type of the component
	Version      string
	Arch         string
	OS           string     // Operating system, e.g., "linux"
	Zone         string     // Download zone, e.g., "cn" or "" for default
	FileName     string     // Filename of the download, e.g., "etcd-v3.5.9-linux-amd64.tar.gz"
	URL          string     // Download URL
	IsArchive    bool       // True if the downloaded file is an archive (.tar.gz, .tgz, .zip)
	BaseDir      string     // Base directory for storing this binary type locally: ${WORK_DIR}/.kubexm/${CLUSTER_NAME}/${Type}/
	ComponentDir string     // Specific directory for this component: ${BaseDir}/${Component}/${Version}/${Arch}/
	FilePath     string     // Full local path to the downloaded file: ${ComponentDir}/${FileName}
}

// knownBinaryDetails maps component names to their specific download attributes.
// This replaces the switch statement from the markdown for easier management.
var knownBinaryDetails = map[string]struct {
	binaryType          BinaryType
	urlTemplate         string
	cnURLTemplate       string // Specific URL template for "cn" zone
	fileNameTemplate    string
	isArchive           bool
	defaultOS           string
	componentNameForDir string // Name to use for the directory structure, if different from map key
}{
	"etcd": {
		binaryType:       ETCD,
		urlTemplate:      "https://github.com/coreos/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/etcd/release/download/{{.Version}}/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		fileNameTemplate: "etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		isArchive:        true,
		defaultOS:        "linux",
	},
	"kubeadm": {
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubeadm",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubeadm", // Official GCS, often works in CN via proxies
		fileNameTemplate: "kubeadm",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kubelet": {
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubelet",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubelet",
		fileNameTemplate: "kubelet",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kubectl": {
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubectl",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubectl",
		fileNameTemplate: "kubectl",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kube-proxy": { // Added from markdown example
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-proxy",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-proxy",
		fileNameTemplate: "kube-proxy",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kube-scheduler": { // Added
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-scheduler",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-scheduler",
		fileNameTemplate: "kube-scheduler",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kube-controller-manager": { // Added
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-controller-manager",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-controller-manager",
		fileNameTemplate: "kube-controller-manager",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kube-apiserver": { // Added
		binaryType:       KUBE,
		urlTemplate:      "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-apiserver",
		cnURLTemplate:    "https://storage.googleapis.com/kubernetes-release/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kube-apiserver",
		fileNameTemplate: "kube-apiserver",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"kubecni": { // CNI Plugins
		binaryType:       CNI,
		urlTemplate:      "https://github.com/containernetworking/plugins/releases/download/{{.Version}}/cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		cnURLTemplate:    "https://containernetworking.pek3b.qingstor.com/plugins/releases/download/{{.Version}}/cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		fileNameTemplate: "cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		isArchive:        true,
		defaultOS:        "linux",
	},
	"helm": {
		binaryType:       HELM,
		urlTemplate:      "https://get.helm.sh/helm-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		cnURLTemplate:    "https://kubernetes-helm.pek3b.qingstor.com/{{.OS}}-{{.Arch}}/{{.Version}}/helm-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz", // Adjusted CN URL
		fileNameTemplate: "helm-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		isArchive:        true,
		defaultOS:        "linux",
	},
	"docker": {
		binaryType:       DOCKER,
		urlTemplate:      "https://download.docker.com/linux/static/stable/{{.ArchAlias}}/docker-{{.Version}}.tgz",
		cnURLTemplate:    "https://mirrors.aliyun.com/docker-ce/linux/static/stable/{{.ArchAlias}}/docker-{{.Version}}.tgz",
		fileNameTemplate: "docker-{{.Version}}.tgz",
		isArchive:        true,
		defaultOS:        "linux", // OS not in template, but good to have
	},
	"cri-dockerd": {
		binaryType:       CRIDOCKERD,
		urlTemplate:      "https://github.com/Mirantis/cri-dockerd/releases/download/v{{.VersionNoV}}/cri-dockerd-{{.VersionNoV}}.{{.Arch}}.tgz", // VersionNoV for cri-dockerd if it doesn't use 'v' prefix in URL
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/cri-dockerd/releases/download/v{{.VersionNoV}}/cri-dockerd-{{.VersionNoV}}.{{.Arch}}.tgz",
		fileNameTemplate: "cri-dockerd-{{.VersionNoV}}.{{.Arch}}.tgz",
		isArchive:        true,
		defaultOS:        "linux",
	},
	"crictl": {
		binaryType:       CRICTL,
		urlTemplate:      "https://github.com/kubernetes-sigs/cri-tools/releases/download/{{.Version}}/crictl-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/cri-tools/releases/download/{{.Version}}/crictl-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		fileNameTemplate: "crictl-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		isArchive:        true,
		defaultOS:        "linux",
	},
	"k3s": {
		binaryType:       K3S,
		urlTemplate:      "https://github.com/k3s-io/k3s/releases/download/{{.VersionWithPlus}}/k3s{{.ArchSuffix}}", // VersionWithPlus (e.g. v1.20.0+k3s1), ArchSuffix (-arm64 or empty)
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/k3s/releases/download/{{.VersionWithPlus}}/linux/{{.Arch}}/k3s", // CN URL might be different format
		fileNameTemplate: "k3s{{.ArchSuffix}}",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"k8e": { // Assuming k8e follows k3s style
		binaryType:       K8E,
		urlTemplate:      "https://github.com/xiaods/k8e/releases/download/{{.VersionWithPlus}}/k8e{{.ArchSuffix}}",
		cnURLTemplate:    "", // No CN URL provided in markdown example
		fileNameTemplate: "k8e{{.ArchSuffix}}",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"registry": { // Local registry binary
		binaryType:          REGISTRY,
		urlTemplate:         "https://github.com/kubesphere/kubekey/releases/download/v2.0.0-alpha.1/registry-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz", // Version is for registry, not kubekey
		cnURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/registry/{{.Version}}/registry-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		fileNameTemplate:    "registry-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		isArchive:           true,
		defaultOS:           "linux",
		componentNameForDir: "registry", // Explicit name for directory
	},
	"harbor": {
		binaryType:          REGISTRY,
		urlTemplate:         "https://github.com/goharbor/harbor/releases/download/{{.Version}}/harbor-offline-installer-{{.Version}}.tgz",
		cnURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/harbor/releases/download/{{.Version}}/harbor-offline-installer-{{.Version}}.tgz",
		fileNameTemplate:    "harbor-offline-installer-{{.Version}}.tgz",
		isArchive:           true,
		defaultOS:           "linux", // OS/Arch not in filename/URL typically
		componentNameForDir: "harbor",
	},
	"compose": { // Docker Compose
		binaryType:          REGISTRY, // Grouping with registry tools for now
		urlTemplate:         "https://github.com/docker/compose/releases/download/{{.Version}}/docker-compose-{{.OS}}-{{.ArchAlias}}",
		cnURLTemplate:       "https://kubernetes-release.pek3b.qingstor.com/docker/compose/releases/download/{{.Version}}/docker-compose-{{.OS}}-{{.ArchAlias}}",
		fileNameTemplate:    "docker-compose-{{.OS}}-{{.ArchAlias}}",
		isArchive:           false, // It's a direct binary
		defaultOS:           "linux",
		componentNameForDir: "compose",
	},
	"containerd": {
		binaryType:       CONTAINERD,
		urlTemplate:      "https://github.com/containerd/containerd/releases/download/v{{.VersionNoV}}/containerd-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/containerd/containerd/releases/download/v{{.VersionNoV}}/containerd-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		fileNameTemplate: "containerd-{{.VersionNoV}}-{{.OS}}-{{.Arch}}.tar.gz",
		isArchive:        true,
		defaultOS:        "linux",
	},
	"runc": {
		binaryType:       RUNC,
		urlTemplate:      "https://github.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		fileNameTemplate: "runc.{{.Arch}}",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"calicoctl": {
		binaryType:       CALICOCTL,
		urlTemplate:      "https://github.com/projectcalico/calico/releases/download/{{.Version}}/calicoctl-{{.OS}}-{{.Arch}}",
		cnURLTemplate:    "https://kubernetes-release.pek3b.qingstor.com/projectcalico/calico/releases/download/{{.Version}}/calicoctl-{{.OS}}-{{.Arch}}",
		fileNameTemplate: "calicoctl-{{.OS}}-{{.Arch}}",
		isArchive:        false,
		defaultOS:        "linux",
	},
	"buildx": { // Docker buildx
		binaryType:       BUILD,
		urlTemplate:      "https://github.com/docker/buildx/releases/download/{{.Version}}/buildx-{{.Version}}.{{.OS}}-{{.Arch}}",
		cnURLTemplate:    "", // No CN provided
		fileNameTemplate: "buildx-{{.Version}}.{{.OS}}-{{.Arch}}",
		isArchive:        false, // Typically a direct binary
		defaultOS:        "linux",
	},
}

// templateData is used for rendering URL and filename templates.
type templateData struct {
	Version         string
	VersionNoV      string // Version without 'v' prefix
	VersionWithPlus string // Version with '+' (for k3s/k8e, e.g. v1.20.0+k3s1)
	Arch            string
	ArchAlias       string // e.g., x86_64 for amd64
	ArchSuffix      string // e.g., -arm64 for k3s arm64, empty otherwise
	OS              string
}

// GetBinaryInfo returns information about a downloadable binary component.
// componentName should be a key from knownBinaryDetails.
// version should be like "v1.2.3" or "1.2.3" or "v1.2.3+k3s1".
// arch should be "amd64" or "arm64". If empty, defaults to host arch.
// zone is "cn" for China region, otherwise uses default URLs.
// workDir and clusterName are used to construct local storage paths.
func GetBinaryInfo(componentName, version, arch, zone, workDir, clusterName string) (*BinaryInfo, error) {
	details, ok := knownBinaryDetails[strings.ToLower(componentName)]
	if !ok {
		return nil, fmt.Errorf("unknown binary component: %s", componentName)
	}

	finalArch := arch
	if finalArch == "" {
		// In a real scenario, this might try to get host arch. For now, default.
		finalArch = "amd64"
	}
	if finalArch == "x86_64" {
		finalArch = "amd64"
	} else if finalArch == "aarch64" {
		finalArch = "arm64"
	}

	finalOS := details.defaultOS
	if finalOS == "" {
		finalOS = "linux" // Ultimate fallback
	}

	td := templateData{
		Version:         version,                 // e.g., v1.2.3 or v1.2.3+k3s1
		VersionNoV:      strings.TrimPrefix(version, "v"), // e.g., 1.2.3 or 1.2.3+k3s1
		VersionWithPlus: version,                 // For k3s/k8e that include it directly
		Arch:            finalArch,
		ArchAlias:       ArchAlias(finalArch),    // e.g., x86_64
		OS:              finalOS,
	}
	if componentName == "k3s" || componentName == "k8e" { // k3s/k8e specific arch suffix in filename/URL
		if finalArch == "arm64" {
			td.ArchSuffix = "-" + finalArch
		} else {
			td.ArchSuffix = "" // amd64 usually has no suffix
		}
	}


	urlTmpl := details.urlTemplate
	if strings.ToLower(zone) == "cn" && details.cnURLTemplate != "" {
		urlTmpl = details.cnURLTemplate
	}

	fileName, err := RenderTemplate(details.fileNameTemplate, td)
	if err != nil {
		return nil, fmt.Errorf("failed to render filename for %s: %w", componentName, err)
	}

	// Pass filename to URL template if needed (some templates might use {{.FileName}})
	// Not standard, but was in original markdown's NewKubeBinary struct. Let's avoid if not needed.
	// For now, assume URL templates don't need {{.FileName}}.

	downloadURL, err := RenderTemplate(urlTmpl, td)
	if err != nil {
		return nil, fmt.Errorf("failed to render download URL for %s: %w", componentName, err)
	}

	// Determine BaseDir and ComponentDir for local storage
	// workdir/.kubexm/${cluster_name}/${type}/${component_id_or_name}/${version}/${arch}/
	kubexmRoot := filepath.Join(workDir, ".kubexm")
	if clusterName == "" {
		return nil, fmt.Errorf("clusterName cannot be empty for generating binary path")
	}
	clusterPath := filepath.Join(kubexmRoot, clusterName)

	binaryTypeDirName := string(details.binaryType) // This is "etcd", "kubernetes", "container_runtime"

	// Path structure from 21-其他说明.md:
	// workdir/.kubexm/${cluster_name}/${type}/${version}/${arch}/
	// workdir/.kubexm/${cluster_name}/container_runtime/${container_runtime_name}/${container_runtime_version}/${arch}/

	var componentVersionSpecificDir string
	// For container_runtime, there's an additional sub-directory for the component name itself.
	if details.binaryType == CONTAINERD || details.binaryType == DOCKER || details.binaryType == RUNC || details.binaryType == CRIDOCKERD {
		// Use componentNameForDir if specified, otherwise use componentName
		componentDirNamePart := details.componentNameForDir
		if componentDirNamePart == "" {
			componentDirNamePart = componentName
		}
		componentVersionSpecificDir = filepath.Join(clusterPath, binaryTypeDirName, componentDirNamePart, version, finalArch)
	} else {
		// For etcd, kubernetes, etc., the structure is ${type}/${version}/${arch}
		componentVersionSpecificDir = filepath.Join(clusterPath, binaryTypeDirName, version, finalArch)
	}

	filePath := filepath.Join(componentVersionSpecificDir, fileName)
	baseDir := filepath.Join(clusterPath, binaryTypeDirName) // Base for the type, e.g. .../etcd/ or .../kubernetes/

	return &BinaryInfo{
		Component:    componentName,
		Type:         details.binaryType,
		Version:      version, // Store original version string
		Arch:         finalArch,
		OS:           finalOS,
		Zone:         zone,
		FileName:     fileName,
		URL:          downloadURL,
		IsArchive:    details.isArchive,
		BaseDir:      baseDir,                     // e.g., /work/.kubexm/mycluster/etcd
		ComponentDir: componentVersionSpecificDir, // e.g., /work/.kubexm/mycluster/etcd/v3.5.9/amd64
		FilePath:     filePath,                    // e.g., /work/.kubexm/mycluster/etcd/v3.5.9/amd64/etcd-v3.5.9-linux-amd64.tar.gz
	}, nil
}

// RenderTemplate executes a Go template with the given data.
// Moved from original util/template.go to be co-located if it's primarily for this.
// If it's generic, it should stay in a general util file. For now, assume it's here.
// This is also defined in pkg/resource/remote_binary.go as common.RenderTemplate
// We should use a single definition. For now, let's assume common.RenderTemplate exists.
// For this step, I will remove this local definition and assume a common one.
/*
func RenderTemplate(tmplStr string, data interface{}) (string, error) {
	tmpl, err := template.New(fmt.Sprintf("binaryInfoTmpl-%d", rand.Int())).Parse(tmplStr) // Unique name for template
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
*/

// GetZone retrieves the download zone from the KXZONE environment variable.
// Returns "cn" if KXZONE is "cn", otherwise returns an empty string (default global zone).
func GetZone() string {
	if strings.ToLower(os.Getenv("KXZONE")) == "cn" {
		return "cn"
	}
	return ""
}

// ArchAlias returns the architecture alias typically used in download URLs (e.g., "x86_64" for "amd64").
// This is based on the util.ArchAlias function mentioned in the markdown.
func ArchAlias(arch string) string {
	switch strings.ToLower(arch) {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64" // Or "arm64v8" depending on the specific URL requirement. For Docker, x86_64/aarch64 is common.
	default:
		return arch
	}
}

// RenderTemplate is defined in template.go
