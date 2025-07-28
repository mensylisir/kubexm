package binary

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util" // 确保引入您的 util 包
	"path/filepath"
	"strings"
)

// --- 基础类型和常量定义 ---

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

// --- 核心对象模型: Binary ---

// Binary 对象代表一个二进制文件的所有相关信息，并封装了所有计算逻辑。
// 这是一个不应被外部直接创建的内部模型，应通过 BinaryProvider 获取。
type Binary struct {
	// --- 核心属性 ---
	ComponentName string
	Version       string
	Arch          string
	Zone          string

	// --- 元数据 (从 details map 注入) ---
	meta BinaryDetailSpec

	// --- 路径参数 (用于构建路径) ---
	workDir     string
	clusterName string
}

// --- 公共方法 ---

// URL 返回计算出的最终下载地址。
func (b *Binary) URL() string {
	urlTmpl := b.meta.URLTemplate
	if strings.ToLower(b.Zone) == "cn" && b.meta.CNURLTemplate != "" {
		urlTmpl = b.meta.CNURLTemplate
	}
	url, _ := util.RenderTemplate(urlTmpl, b.templateData())
	return url
}

// FileName 返回计算出的最终文件名。
func (b *Binary) FileName() string {
	name, _ := util.RenderTemplate(b.meta.FileNameTemplate, b.templateData())
	return name
}

// FilePath 返回计算出的最终本地存储路径。
func (b *Binary) FilePath() string {
	return filepath.Join(b.componentDir(), b.FileName())
}

// IsArchive 返回该二进制文件是否是一个压缩包。
func (b *Binary) IsArchive() bool {
	return b.meta.IsArchive
}

// Type 返回该二进制文件的类型。
func (b *Binary) Type() BinaryType {
	return b.meta.BinaryType
}

// Checksum 返回预期的校验和。
func (b *Binary) Checksum() string {
	return b.meta.ExpectedChecksum
}

// --- 私有辅助方法 ---

// templateData 准备用于渲染模板的数据。
func (b *Binary) templateData() interface{} {
	versionNoV := strings.TrimPrefix(b.Version, "v")
	versionWithPlus := b.Version
	if strings.HasPrefix(b.Version, "v") && (b.ComponentName == ComponentK3s || b.ComponentName == ComponentK8e) {
		versionWithPlus = strings.Replace(b.Version, "v", "v-", 1)
	}

	archSuffix := ""
	if b.ComponentName == ComponentK3s || b.ComponentName == ComponentK8e {
		if b.Arch == "arm64" {
			archSuffix = "-" + b.Arch
		}
	}

	return struct {
		Version         string
		VersionNoV      string
		VersionWithPlus string
		Arch            string
		ArchAlias       string
		ArchSuffix      string
		OS              string
	}{
		Version:         b.Version,
		VersionNoV:      versionNoV,
		VersionWithPlus: versionWithPlus,
		Arch:            b.Arch,
		ArchAlias:       ArchAlias(b.Arch),
		ArchSuffix:      archSuffix,
		OS:              b.meta.DefaultOS,
	}
}

// componentDir 计算组件的特定版本和架构的存储目录。
func (b *Binary) componentDir() string {
	if b.workDir == "" || b.clusterName == "" {
		// 防止因未初始化路径参数而 panic
		return ""
	}
	kubexmRoot := filepath.Join(b.workDir, common.KubexmRootDirName)
	clusterBaseDir := filepath.Join(kubexmRoot, b.clusterName)
	var typeSpecificBaseDir string

	switch b.meta.BinaryType {
	case ETCD:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameEtcd)
	case KUBE, K3S, K8E:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameKubernetes)
	case CNI, CALICOCTL:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameKubernetes, common.DefaultCNIDir)
	case CONTAINERD, DOCKER, RUNC, CRIDOCKERD, CRICTL:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, DirNameContainerRuntime)
	case HELM, BUILD, REGISTRY:
		typeSpecificBaseDir = filepath.Join(clusterBaseDir, string(b.meta.BinaryType))
	default:
		return filepath.Join(clusterBaseDir, "unknown")
	}

	compDirName := b.meta.ComponentNameForDir
	if compDirName == "" {
		compDirName = b.ComponentName
	}

	// 路径构建逻辑拆分以增加清晰度
	switch b.meta.BinaryType {
	case ETCD, KUBE, K3S, K8E:
		return filepath.Join(typeSpecificBaseDir, b.Version, b.Arch)
	case CNI, CALICOCTL, CONTAINERD, DOCKER, RUNC, CRIDOCKERD, CRICTL, HELM, BUILD, REGISTRY:
		return filepath.Join(typeSpecificBaseDir, compDirName, b.Version, b.Arch)
	default:
		// Fallback for safety
		return filepath.Join(typeSpecificBaseDir, compDirName, b.Version, b.Arch)
	}
}

// ArchAlias 是一个辅助函数，您之前的代码中已有
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
