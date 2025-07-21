package helpers

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
)

type BinaryType string

const (
	ETCD       BinaryType = "etcd"
	KUBE       BinaryType = "kubernetes"
	CNI        BinaryType = "cni"
	HELM       BinaryType = "helm"
	DOCKER     BinaryType = "docker"
	CRIDOCKERD BinaryType = "cri-dockerd"
	CRICTL     BinaryType = "crictl"
	K3S        BinaryType = "k3s"
	K8E        BinaryType = "k8e"
	REGISTRY   BinaryType = "registry"
	BUILD      BinaryType = "build"
	CONTAINERD BinaryType = "containerd"
	RUNC       BinaryType = "runc"
	CALICOCTL  BinaryType = "calicoctl"
	UNKNOWN    BinaryType = "unknown"
)

const (
	ComponentEtcd                  = "etcd"
	ComponentKubeadm               = "kubeadm"
	ComponentKubelet               = "kubelet"
	ComponentKubectl               = "kubectl"
	ComponentKubeProxy             = "kube-proxy"
	ComponentKubeScheduler         = "kube-scheduler"
	ComponentKubeControllerManager = "kube-controller-manager"
	ComponentKubeApiServer         = "kube-apiserver"
	ComponentKubeCNI               = "kubecni"
	ComponentHelm                  = "helm"
	ComponentDocker                = "docker"
	ComponentCriDockerd            = "cri-dockerd"
	ComponentCriCtl                = "crictl"
	ComponentK3s                   = "k3s"
	ComponentK8e                   = "k8e"
	ComponentRegistry              = "registry"
	ComponentHarbor                = "harbor"
	ComponentCompose               = "compose"
	ComponentContainerd            = "containerd"
	ComponentRunc                  = "runc"
	ComponentCalicoCtl             = "calicoctl"
	ComponentBuildx                = "buildx"
)

const (
	DirNameCerts            = "certs"
	DirNameEtcd             = common.DefaultEtcdDir
	DirNameContainerRuntime = common.DefaultContainerRuntimeDir
	DirNameKubernetes       = common.DefaultKubernetesDir
)

type BinaryInfo struct {
	Component            string
	Type                 BinaryType
	Version              string
	Arch                 string
	OS                   string
	Zone                 string
	FileName             string
	URL                  string
	IsArchive            bool
	BaseDir              string
	ComponentDir         string
	FilePath             string
	ExpectedChecksum     string
	ExpectedChecksumType string
}

type BinaryDetailSpec struct {
	BinaryType           BinaryType
	URLTemplate          string
	CNURLTemplate        string
	FileNameTemplate     string
	IsArchive            bool
	DefaultOS            string
	ComponentNameForDir  string
	ExpectedChecksum     string
	ExpectedChecksumType string
}

type BinaryProvider struct {
	details map[string]BinaryDetailSpec
}

func NewBinaryProvider() *BinaryProvider {
	return &BinaryProvider{
		details: defaultKnownBinaryDetails,
	}
}

var defaultKnownBinaryDetails = map[string]BinaryDetailSpec{
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

type templateData struct {
	Version         string
	VersionNoV      string
	VersionWithPlus string
	Arch            string
	ArchAlias       string
	ArchSuffix      string
	OS              string
}

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
		finalArch = runtime.GOARCH
	}
	switch finalArch {
	case common.ArchX8664:
		finalArch = common.ArchAMD64
	case common.ArchAarch64:
		finalArch = common.ArchARM64
	}

	finalOS := details.DefaultOS
	if finalOS == "" {
		finalOS = common.OSLinux
	}

	versionNoV := strings.TrimPrefix(version, "v")
	versionWithPlus := version

	archSuffix := ""
	if componentName == ComponentK3s || componentName == ComponentK8e {
		if finalArch == "arm64" {
			archSuffix = "-" + finalArch
		}
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

	urlTmplToUse := details.URLTemplate
	if strings.ToLower(zone) == "cn" && details.CNURLTemplate != "" {
		urlTmplToUse = details.CNURLTemplate
	}

	fileName, err := util.RenderTemplate(details.FileNameTemplate, td)
	if err != nil {
		return nil, fmt.Errorf("failed to render filename for %s (template: '%s'): %w", componentName, details.FileNameTemplate, err) // Corrected
	}

	downloadURL, err := util.RenderTemplate(urlTmplToUse, td)
	if err != nil {
		return nil, fmt.Errorf("failed to render download URL for %s (template: '%s'): %w", componentName, urlTmplToUse, err)
	}
	if workDir == "" {
		return nil, fmt.Errorf("workDir cannot be empty for generating binary path")
	}
	if clusterName == "" {
		return nil, fmt.Errorf("clusterName cannot be empty for generating binary path")
	}

	kubexmRoot := filepath.Join(workDir, common.KubexmRootDirName)
	clusterBaseDir := filepath.Join(kubexmRoot, clusterName)

	var typeSpecificBaseDir string
	var componentVersionSpecificDir string

	switch details.BinaryType {
	case ETCD:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameEtcd)
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, version, finalArch)
	case KUBE, K3S, K8E:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameKubernetes)
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, version, finalArch)
	case CNI, CALICOCTL:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameKubernetes, common.DefaultCNIDir)
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, componentName, version, finalArch)
	case CONTAINERD, DOCKER, RUNC, CRIDOCKERD, CRICTL:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameContainerRuntime)
		compDirName := details.ComponentNameForDir
		if compDirName == "" {
			compDirName = componentName
		}
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, compDirName, version, finalArch)
	case HELM, BUILD, REGISTRY:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, string(details.BinaryType))
		compDirName := details.ComponentNameForDir
		if compDirName == "" {
			compDirName = componentName
		}
		componentVersionSpecificDir = filepath.Join(typeSpecificBaseDir, compDirName, version, finalArch)

	default:
		return nil, fmt.Errorf("unhandled binary type '%s' for path construction", details.BinaryType)
	}

	filePath := filepath.Join(componentVersionSpecificDir, fileName)

	return &BinaryInfo{
		Component:            componentName,
		Type:                 details.BinaryType,
		Version:              version,
		Arch:                 finalArch,
		OS:                   finalOS,
		Zone:                 zone,
		FileName:             fileName,
		URL:                  downloadURL,
		IsArchive:            details.IsArchive,
		BaseDir:              typeSpecificBaseDir,
		ComponentDir:         componentVersionSpecificDir,
		FilePath:             filePath,
		ExpectedChecksum:     details.ExpectedChecksum,
		ExpectedChecksumType: details.ExpectedChecksumType,
	}, nil
}

func GetZone() string {
	if strings.ToLower(os.Getenv("KXZONE")) == "cn" {
		return "cn"
	}
	return ""
}

func ArchAlias(arch string) string {
	switch strings.ToLower(arch) {
	case common.ArchAMD64:
		return common.ArchX8664
	case common.ArchARM64:
		return common.ArchAarch64
	default:
		return arch
	}
}
