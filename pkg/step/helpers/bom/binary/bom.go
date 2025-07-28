package binary

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
)

// ComponentBinaryBOM (Bill of Materials) 存储了一个二进制组件的版本与 K8s 的兼容关系。
type ComponentBinaryBOM struct {
	KubeVersionConstraints string
	Version                string
}

// componentBinaryBOMs 是二进制组件的核心物料清单。
// 注意: 此清单不包含 Kubernetes 核心组件 (kubelet, kubeadm 等)。
var componentBinaryBOMs = map[string][]ComponentBinaryBOM{
	// --- K8s 依赖型组件 ---
	ComponentEtcd: {
		{KubeVersionConstraints: ">= 1.29.0", Version: "v3.5.10-0"},
		{KubeVersionConstraints: ">= 1.28.0, < 1.29.0", Version: "v3.5.9-0"},
		{KubeVersionConstraints: "< 1.28.0", Version: "v3.5.7-0"},
	},
	ComponentContainerd: {
		{KubeVersionConstraints: ">= 1.26.0", Version: "1.7.13"},
		{KubeVersionConstraints: "< 1.26.0", Version: "1.6.28"},
	},
	ComponentKubeCNI: {
		{KubeVersionConstraints: ">= 1.24.0", Version: "v1.4.0"},
		{KubeVersionConstraints: "< 1.24.0", Version: "v1.2.0"},
	},
	ComponentRunc: {
		{KubeVersionConstraints: ">= 1.26.0", Version: "v1.1.12"},
		{KubeVersionConstraints: "< 1.26.0", Version: "v1.1.9"},
	},
	ComponentCriCtl: {
		{KubeVersionConstraints: ">= 1.29.0", Version: "v1.29.0"},
		{KubeVersionConstraints: "< 1.29.0", Version: "v1.28.0"},
	},
	ComponentCalicoCtl: {
		{KubeVersionConstraints: ">= 1.28.0, < 1.31.0", Version: "v3.28.0"},
		{KubeVersionConstraints: "< 1.28.0", Version: "v3.27.3"},
	},
	ComponentDocker: {
		{KubeVersionConstraints: ">= 1.27.0", Version: "26.1.1"},
		{KubeVersionConstraints: "< 1.27.0", Version: "25.0.5"},
	},
	ComponentCriDockerd: {
		{KubeVersionConstraints: ">= 1.27.0", Version: "0.3.10"},
		{KubeVersionConstraints: "< 1.27.0, >= 1.24.0", Version: "0.3.1"},
	},

	// --- 独立型组件 ---
	ComponentHelm: {
		{KubeVersionConstraints: ">= 0.0.0", Version: "v3.14.4"},
	},
	ComponentRegistry: {
		{KubeVersionConstraints: ">= 0.0.0", Version: "2.8.3"},
	},
	ComponentHarbor: {
		{KubeVersionConstraints: ">= 0.0.0", Version: "v2.11.0"},
	},
	ComponentCompose: {
		{KubeVersionConstraints: ">= 0.0.0", Version: "v2.27.0"},
	},
	ComponentBuildx: {
		{KubeVersionConstraints: ">= 0.0.0", Version: "v0.14.0"},
	},
}

// getBinaryVersionFromBOM 是一个内部辅助函数，
// 它的唯一职责就是根据 K8s 版本从 BOM 中查找推荐的组件版本。
func getBinaryVersionFromBOM(componentName string, kubeVersionStr string) string {
	if componentName == "" {
		return ""
	}

	if kubeVersionStr == "" {
		kubeVersionStr = "0.0.0"
	}

	k8sVersion, err := semver.NewVersion(kubeVersionStr)
	if err != nil {
		fmt.Printf("Warning: could not parse kubernetes version '%s' for binary BOM lookup: %v\n", kubeVersionStr, err)
		if bomList, ok := componentBinaryBOMs[componentName]; ok {
			for _, entry := range bomList {
				if entry.KubeVersionConstraints == ">= 0.0.0" {
					return entry.Version
				}
			}
		}
		return ""
	}

	componentBOMList, ok := componentBinaryBOMs[componentName]
	if !ok {
		return ""
	}

	for _, bomEntry := range componentBOMList {
		constraints, err := semver.NewConstraint(bomEntry.KubeVersionConstraints)
		if err != nil {
			fmt.Printf("Error: invalid version constraint in binary BOM for %s: '%s'. Skipping.\n", componentName, bomEntry.KubeVersionConstraints)
			continue
		}
		if constraints.Check(k8sVersion) {
			return bomEntry.Version
		}
	}

	fmt.Printf("Warning: no compatible binary version in BOM for component '%s' with K8s '%s'\n", componentName, kubeVersionStr)
	return ""
}
