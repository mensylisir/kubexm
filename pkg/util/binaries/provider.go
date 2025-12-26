package binaries

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// BinaryProvider 封装了所有与获取二进制文件信息相关的业务逻辑。
type BinaryProvider struct {
	ctx     runtime.ExecutionContext
	details map[string]BinaryDetailSpec
}

// NewBinaryProvider 创建一个新的 BinaryProvider 实例。
func NewBinaryProvider(ctx runtime.ExecutionContext) *BinaryProvider {
	return &BinaryProvider{
		ctx:     ctx,
		details: defaultKnownBinaryDetails, // 引用在 binary_metadata.go 中定义的元数据
	}
}

// GetBinary 是一站式的函数，用于获取二进制文件的所有信息。
func (p *BinaryProvider) GetBinary(name, arch string) (*Binary, error) {
	cfg := p.ctx.GetClusterConfig()
	if cfg == nil {
		return nil, fmt.Errorf("cluster config is not available in context")
	}

	// 1. 判断组件是否在当前配置下启用
	isEnabled, err := p.isBinaryEnabled(name)
	if err != nil {
		return nil, err
	}
	if !isEnabled {
		return nil, nil // 返回 nil, nil 表示“已禁用，非错误”
	}

	// 2. 确定最终版本号
	userVersion := p.getUserSpecifiedVersion(name)
	kubeVersion := cfg.Spec.Kubernetes.Version

	finalVersion := userVersion
	var finalChecksum string

	if userVersion != "" {
		finalVersion = userVersion
		p.ctx.GetLogger().Debugf("Using user-specified version '%s' for component '%s'. Checksum will not be available from BOM.", finalVersion, name)
	} else {
		switch name {
		case ComponentKubeadm, ComponentKubelet, ComponentKubectl, ComponentKubeProxy,
			ComponentKubeScheduler, ComponentKubeControllerManager, ComponentKubeApiServer,
			ComponentK3s, ComponentK8e:
			finalVersion = kubeVersion
		default:
			bomEntry := getBinaryBOMEntry(name, kubeVersion)
			if bomEntry != nil {
				finalVersion = bomEntry.Version
				if bomEntry.Checksums != nil {
					finalChecksum = bomEntry.Checksums[arch]
				}
			}
		}
	}
	if finalVersion == "" {
		return nil, fmt.Errorf("version for component '%s' is not specified and could not be determined", name)
	}

	if finalChecksum == "" {
		p.ctx.GetLogger().Warnf("Checksum for component %s (version: %s, arch: %s) not found in BOM. Verification will be skipped.", name, finalVersion, arch)
	}

	meta, ok := p.details[name]
	if !ok {
		return nil, fmt.Errorf("unknown binary component in metadata details: %s", name)
	}

	return &Binary{
		ComponentName: name,
		Version:       finalVersion,
		Arch:          arch,
		Zone:          GetZone(),
		meta:          meta,
		workDir:       p.ctx.GetGlobalWorkDir(),
		clusterName:   cfg.Name,
	}, nil
}

func (p *BinaryProvider) GetBinaries(arch string) ([]*Binary, error) {
	var enabledBinaries []*Binary
	allBinaryNames := p.getManagedBinaryNames()

	for _, name := range allBinaryNames {
		binary, err := p.GetBinary(name, arch)
		if err != nil {
			return nil, fmt.Errorf("failed to get binary info for %s: %w", name, err)
		}
		if binary != nil {
			enabledBinaries = append(enabledBinaries, binary)
		}
	}
	return enabledBinaries, nil
}

func (p *BinaryProvider) getManagedBinaryNames() []string {
	names := make([]string, 0, len(p.details))
	for name := range p.details {
		names = append(names, name)
	}
	return names
}

// getUserSpecifiedVersion 从 ClusterConfig 中获取用户为特定组件指定的版本。
func (p *BinaryProvider) getUserSpecifiedVersion(name string) string {
	cfg := p.ctx.GetClusterConfig().Spec

	switch name {
	case ComponentEtcd:
		if cfg.Etcd != nil {
			return cfg.Etcd.Version
		}
	case ComponentContainerd:
		if cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Type == "containerd" {
			return cfg.Kubernetes.ContainerRuntime.Containerd.Version
		}
	case ComponentDocker:
		if cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Type == "docker" {
			return cfg.Kubernetes.ContainerRuntime.Docker.Version
		}
	case ComponentCrio:
		if cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Type == "crio" {
			return cfg.Kubernetes.ContainerRuntime.Crio.Version
		}
		// --- 工具链和应用 ---
		//case ComponentHelm:
		//	if cfg.Tools != nil {
		//		return cfg.Tools.HelmVersion
		//	}
		//case ComponentCalicoCtl:
		//	if cfg.Tools != nil {
		//		return cfg.Tools.CalicoctlVersion
		//	}
		//case ComponentCompose:
		//	if cfg.Tools != nil {
		//		return cfg.Tools.ComposeVersion
		//	}
		//case ComponentBuildx:
		//	if cfg.Tools != nil {
		//		return cfg.Tools.BuildxVersion
		//	}
		//case ComponentHarbor:
		//	if cfg.Registry.LocalDeployment != nil && cfg.Registry.LocalDeployment.Type == "harbor" {
		//		return cfg.Registry.LocalDeployment.Version
		//	}
		//case ComponentRegistry:
		//	if cfg.Registry.LocalDeployment != nil && cfg.Registry.LocalDeployment.Type == "registry" {
		//		return cfg.Registry.LocalDeployment.Version
		//	}
	}
	return ""
}

// isBinaryEnabled 封装启用/禁用逻辑。
func (p *BinaryProvider) isBinaryEnabled(name string) (bool, error) {
	cfg := p.ctx.GetClusterConfig().Spec

	switch name {
	// --- 核心组件, 总是启用 ---
	case ComponentKubeadm, ComponentKubelet, ComponentKubectl,
		ComponentKubeScheduler, ComponentKubeControllerManager, ComponentKubeApiServer,
		ComponentKubeCNI:
		return true, nil

	case ComponentEtcd:
		if cfg.Etcd == nil {
			return false, fmt.Errorf("etcd configuration is missing")
		}
		return strings.EqualFold(cfg.Etcd.Type, string(common.EtcdDeploymentTypeKubexm)), nil

	case ComponentKubeProxy:
		return cfg.Kubernetes.KubeProxy == nil || cfg.Kubernetes.KubeProxy.Enable == nil || *cfg.Kubernetes.KubeProxy.Enable, nil

	// --- 容器运行时相关 ---
	case ComponentContainerd, ComponentRunc, ComponentCriCtl:
		// 只要配置了容器运行时（非空），这些都是基础组件
		return cfg.Kubernetes.ContainerRuntime != nil, nil

	case ComponentDocker:
		return cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Type == "docker", nil

	case ComponentCriDockerd:
		isDocker := cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Type == "docker"
		if !isDocker {
			return false, nil
		}
		// 只有 K8s 版本 >= 1.24 才需要 cri-dockerd
		v1_24, _ := semver.NewConstraint(">= 1.24.0")
		k8sVersion, err := semver.NewVersion(cfg.Kubernetes.Version)
		if err != nil {
			return false, fmt.Errorf("invalid kubernetes version for cri-dockerd check: %w", err)
		}
		return v1_24.Check(k8sVersion), nil

	case ComponentCrio: // <-- 新增
		return cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Type == "crio", nil

	case ComponentCalicoCtl:
		return cfg.Network.Plugin == string(common.CNITypeCalico), nil

	// --- 工具链, 通常总是需要下载以备不时之需 ---
	case ComponentHelm, ComponentCompose, ComponentBuildx:
		return true, nil

	// --- 本地部署的应用 ---
	case ComponentHarbor:
		return cfg.Registry.LocalDeployment != nil && cfg.Registry.LocalDeployment.Type == "harbor", nil
	case ComponentRegistry:
		return cfg.Registry.LocalDeployment != nil && cfg.Registry.LocalDeployment.Type == "registry", nil

	// --- 特殊发行版 ---
	case ComponentK3s, ComponentK8e:
		// 这里的判断逻辑取决于您如何在 spec 中定义部署模式
		// 假设有一个字段 DeploymentMode
		// return cfg.DeploymentMode == name
		return false, nil // 默认禁用，除非有明确的模式选择

	default:
		// 对于未明确分类的组件，默认不启用，避免下载不必要的文件
		return false, fmt.Errorf("enablement check for component '%s' is not implemented", name)
	}
}

// GetZone a helper function to get the zone from environment variables.
func GetZone() string {
	if strings.ToLower(os.Getenv("KXZONE")) == "cn" {
		return "cn"
	}
	return ""
}
