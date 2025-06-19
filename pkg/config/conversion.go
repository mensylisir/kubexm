package config

import (
	"fmt"
	// Using placeholders for module name. Replace with actual module name.
	"{{MODULE_NAME}}/pkg/apis/kubexms/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// We are in package config, so config.Cluster is just Cluster, etc.
)

// ToV1Alpha1Cluster converts a config.Cluster object (from YAML parsing)
// to a v1alpha1.Cluster object (Kubernetes API type).
func ToV1Alpha1Cluster(cfg *Cluster) (*v1alpha1.Cluster, error) {
	if cfg == nil {
		return nil, fmt.Errorf("input config.Cluster is nil")
	}

	// Since config.ClusterSpec now directly uses v1alpha1 types for its fields,
	// the conversion is primarily about constructing the v1alpha1.Cluster object
	// and assigning the already-typed Spec.
	v1Cluster := &v1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: cfg.APIVersion,
			Kind:       cfg.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cfg.Metadata.Name,
			// TODO: If config.Metadata includes more fields like Annotations or Labels,
			// they would need to be mapped here to v1Cluster.ObjectMeta.
		},
		// The cfg.Spec is now directly assignable if its type is v1alpha1.ClusterSpec
		// However, the types in config.ClusterSpec are pointers to v1alpha1 types or slices of v1alpha1 types.
		// So, we create v1alpha1.ClusterSpec and assign fields.
		Spec: v1alpha1.ClusterSpec{
			Global:               cfg.Spec.Global,
			Hosts:                cfg.Spec.Hosts,
			ContainerRuntime:     cfg.Spec.ContainerRuntime,
			Containerd:           cfg.Spec.Containerd,
			Etcd:                 cfg.Spec.Etcd,
			Kubernetes:           cfg.Spec.Kubernetes,
			Network:              cfg.Spec.Network,
			HighAvailability:     cfg.Spec.HighAvailability,
			Preflight:            cfg.Spec.Preflight, // Renamed from PreflightConfig
			Kernel:               cfg.Spec.Kernel,    // Renamed from KernelConfig
			Storage:              cfg.Spec.Storage,
			Registry:             cfg.Spec.Registry,
			OS:                   cfg.Spec.OS,
			Addons:               cfg.Spec.Addons,
			RoleGroups:           cfg.Spec.RoleGroups,
			ControlPlaneEndpoint: cfg.Spec.ControlPlaneEndpoint,
			System:               cfg.Spec.System,
			// HostsCount is not part of spec, it's a printcolumn helper in v1alpha1
		},
	}

	// Validate TypeMeta (optional, but good practice)
	if v1Cluster.APIVersion == "" || v1Cluster.Kind == "" {
		// Default if necessary, or rely on v1alpha1.Cluster's own defaulting.
		// For now, assume they are correctly populated from YAML.
	}


	return v1Cluster, nil
}
