package config

import (
	// Import v1alpha1 types
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	// "time" // No longer needed as local structs using it are removed
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // No longer needed here
)

// APIVersion and Kind are standard fields for Kubernetes-style configuration objects.
// These constants can be used by the calling code to verify the parsed TypeMeta.
const (
	DefaultAPIVersion = "kubexms.io/v1alpha1"
	ClusterKind       = "Cluster"
	// Add other Kinds if more top-level config objects are envisioned
)

// Cluster is a type alias for v1alpha1.Cluster.
// This allows pkg/config to provide a parsing entry point that directly
// yields the API type, leveraging the yaml tags defined in the v1alpha1 package.
type Cluster v1alpha1.Cluster

// Note: The local definitions for config.Metadata, config.ClusterSpec,
// PreflightConfigSpec, KernelConfigSpec, and all other previously defined
// spec/holder structs (GlobalSpec, HostSpec, RoleGroupsSpec, etc.) have been removed.
// The YAML parsing will now directly populate the fields of v1alpha1.Cluster,
// including its embedded TypeMeta, ObjectMeta, and its Spec (v1alpha1.ClusterSpec),
// using the yaml tags defined in the v1alpha1 types.
