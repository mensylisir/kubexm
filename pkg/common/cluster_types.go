package common

// General cluster operation types.
// These are now preferably defined with more specific types in types.go (e.g., KubernetesDeploymentType).
// These string constants are kept for broader, general-purpose use or backward compatibility during refactoring.
const (
	// ClusterTypeKubeXM indicates a cluster where core components are deployed as binaries.
	// Prefer using KubernetesDeploymentTypeKubexm from types.go for typed fields.
	ClusterTypeKubeXM = "kubexm"
	// ClusterTypeKubeadm indicates a cluster where core components are deployed via Kubeadm.
	// Prefer using KubernetesDeploymentTypeKubeadm from types.go for typed fields.
	ClusterTypeKubeadm = "kubeadm"
)
