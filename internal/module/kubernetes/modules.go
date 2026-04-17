package kubernetes

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	kubeadm "github.com/mensylisir/kubexm/internal/module/kubernetes/kubeadm"
	kubexm "github.com/mensylisir/kubexm/internal/module/kubernetes/kubexm"
)

// NewControlPlaneModuleForType selects the appropriate control plane module for the given kubernetes type.
func NewControlPlaneModuleForType(kubeType string) module.Module {
	if kubeType == "" {
		kubeType = string(common.KubernetesDeploymentTypeKubeadm)
	}

	switch common.KubernetesDeploymentType(kubeType) {
	case common.KubernetesDeploymentTypeKubeadm:
		return kubeadm.NewKubeadmControlPlaneModule()
	case common.KubernetesDeploymentTypeKubexm:
		return kubexm.NewKubexmControlPlaneModule()
	default:
		return kubeadm.NewKubeadmControlPlaneModule()
	}
}

// NewWorkerModuleForType selects the appropriate worker module for the given kubernetes type.
func NewWorkerModuleForType(kubeType string) module.Module {
	if kubeType == "" {
		kubeType = string(common.KubernetesDeploymentTypeKubeadm)
	}

	switch common.KubernetesDeploymentType(kubeType) {
	case common.KubernetesDeploymentTypeKubeadm:
		return kubeadm.NewKubeadmWorkerModule()
	case common.KubernetesDeploymentTypeKubexm:
		return kubexm.NewKubexmWorkerModule()
	default:
		return kubeadm.NewKubeadmWorkerModule()
	}
}
