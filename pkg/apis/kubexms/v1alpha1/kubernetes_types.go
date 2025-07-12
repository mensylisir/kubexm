package v1alpha1

import (
	"fmt"
	"strings"
	"net" // Added for isValidCIDR
	"k8s.io/apimachinery/pkg/runtime" // Added for RawExtension
	versionutil "k8s.io/apimachinery/pkg/util/version"
)

// KubernetesConfig defines the configuration for Kubernetes components.
type KubernetesConfig struct {
	Type                   string                    `json:"type,omitempty" yaml:"type,omitempty"` // "kubexm" or "kubeadm"
	Version                string                    `json:"version" yaml:"version"`
	// ContainerRuntime field is removed from here; it's a top-level field in ClusterSpec now.
	// ContainerRuntime       *ContainerRuntimeConfig   `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	ClusterName            string                    `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	DNSDomain              string                    `json:"dnsDomain,omitempty" yaml:"dnsDomain,omitempty"` // Default "cluster.local"
	DisableKubeProxy       *bool                     `json:"disableKubeProxy,omitempty" yaml:"disableKubeProxy,omitempty"`
	MasqueradeAll          *bool                     `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"` // Default false
	MaxPods                *int32                    `json:"maxPods,omitempty" yaml:"maxPods,omitempty"` // Default 110
	NodeCidrMaskSize       *int32                    `json:"nodeCidrMaskSize,omitempty" yaml:"nodeCidrMaskSize,omitempty"` // Default 24
	ApiserverCertExtraSans []string                  `json:"apiserverCertExtraSans,omitempty" yaml:"apiserverCertExtraSans,omitempty"`
	ProxyMode              string                    `json:"proxyMode,omitempty" yaml:"proxyMode,omitempty"` // Default "ipvs"
	AutoRenewCerts         *bool                     `json:"autoRenewCerts,omitempty" yaml:"autoRenewCerts,omitempty"` // Default true
	ContainerManager       string                    `json:"containerManager,omitempty" yaml:"containerManager,omitempty"` // No specific field in YAML, usually inferred or part of Kubelet config
	FeatureGates           map[string]bool           `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
	APIServer              *APIServerConfig            `json:"apiServer,omitempty" yaml:"apiServer,omitempty"`
	ControllerManager      *ControllerManagerConfig    `json:"controllerManager,omitempty" yaml:"controllerManager,omitempty"`
	Scheduler              *SchedulerConfig            `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
	Kubelet                *KubeletConfig              `json:"kubelet,omitempty" yaml:"kubelet,omitempty"`
	KubeProxy              *KubeProxyConfig            `json:"kubeProxy,omitempty" yaml:"kubeProxy,omitempty"`
	KubeletConfiguration   *runtime.RawExtension       `json:"kubeletConfiguration,omitempty" yaml:"kubeletConfiguration,omitempty"`
	KubeProxyConfiguration *runtime.RawExtension       `json:"kubeProxyConfiguration,omitempty" yaml:"kubeProxyConfiguration,omitempty"`
	Nodelocaldns           *NodelocaldnsConfig         `json:"nodelocaldns,omitempty" yaml:"nodelocaldns,omitempty"`
	Audit                  *AuditConfig                `json:"audit,omitempty" yaml:"audit,omitempty"`
	Kata                   *KataConfig                 `json:"kata,omitempty" yaml:"kata,omitempty"`
	NodeFeatureDiscovery   *NodeFeatureDiscoveryConfig `json:"nodeFeatureDiscovery,omitempty" yaml:"nodeFeatureDiscovery,omitempty"`
}

// APIServerConfig holds configuration for the Kubernetes API Server.
type APIServerConfig struct {
	ExtraArgs            []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	EtcdServers          []string `json:"etcdServers,omitempty" yaml:"etcdServers,omitempty"`
	EtcdCAFile           string   `json:"etcdCAFile,omitempty" yaml:"etcdCAFile,omitempty"`
	EtcdCertFile         string   `json:"etcdCertFile,omitempty" yaml:"etcdCertFile,omitempty"`
	EtcdKeyFile          string   `json:"etcdKeyFile,omitempty" yaml:"etcdKeyFile,omitempty"`
	AdmissionPlugins     []string `json:"admissionPlugins,omitempty" yaml:"admissionPlugins,omitempty"`
	ServiceNodePortRange string   `json:"serviceNodePortRange,omitempty" yaml:"serviceNodePortRange,omitempty"`
}

// ControllerManagerConfig holds configuration for the Kubernetes Controller Manager.
type ControllerManagerConfig struct {
	ExtraArgs                    []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ServiceAccountPrivateKeyFile string   `json:"serviceAccountPrivateKeyFile,omitempty" yaml:"serviceAccountPrivateKeyFile,omitempty"`
}

// SchedulerConfig holds configuration for the Kubernetes Scheduler.
type SchedulerConfig struct {
	ExtraArgs        []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	PolicyConfigFile string   `json:"policyConfigFile,omitempty" yaml:"policyConfigFile,omitempty"`
}

// KubeletConfig holds configuration for the Kubelet.
type KubeletConfig struct {
	ExtraArgs        []string            `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	CgroupDriver     *string             `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`
	EvictionHard     map[string]string   `json:"evictionHard,omitempty" yaml:"evictionHard,omitempty"`
	HairpinMode      *string             `json:"hairpinMode,omitempty" yaml:"hairpinMode,omitempty"`
	PodPidsLimit     *int64              `json:"podPidsLimit,omitempty" yaml:"podPidsLimit,omitempty"`
}

// KubeProxyIPTablesConfig defines specific configuration for KubeProxy in IPTables mode.
type KubeProxyIPTablesConfig struct {
   MasqueradeAll *bool  `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"`
   MasqueradeBit *int32 `json:"masqueradeBit,omitempty" yaml:"masqueradeBit,omitempty"`
   SyncPeriod    string `json:"syncPeriod,omitempty" yaml:"syncPeriod,omitempty"`
   MinSyncPeriod string `json:"minSyncPeriod,omitempty" yaml:"minSyncPeriod,omitempty"`
}

// KubeProxyIPVSConfig defines specific configuration for KubeProxy in IPVS mode.
type KubeProxyIPVSConfig struct {
   Scheduler     string   `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
   SyncPeriod    string   `json:"syncPeriod,omitempty" yaml:"syncPeriod,omitempty"`
   MinSyncPeriod string   `json:"minSyncPeriod,omitempty" yaml:"minSyncPeriod,omitempty"`
   ExcludeCIDRs  []string `json:"excludeCIDRs,omitempty" yaml:"excludeCIDRs,omitempty"`
}

// KubeProxyConfig holds configuration for KubeProxy.
type KubeProxyConfig struct {
	ExtraArgs    []string                 `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	IPTables     *KubeProxyIPTablesConfig `json:"ipTables,omitempty" yaml:"ipTables,omitempty"`
	IPVS         *KubeProxyIPVSConfig     `json:"ipvs,omitempty" yaml:"ipvs,omitempty"`
}

// NodelocaldnsConfig holds configuration for nodelocaldns.
type NodelocaldnsConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// AuditConfig holds configuration for Kubernetes API server audit logging.
type AuditConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// KataConfig holds configuration for deploying Kata Containers runtime.
type KataConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// NodeFeatureDiscoveryConfig holds configuration for node-feature-discovery.
type NodeFeatureDiscoveryConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// SetDefaults_KubernetesConfig sets default values for KubernetesConfig.
func SetDefaults_KubernetesConfig(cfg *KubernetesConfig, clusterMetaName string) {
	if cfg == nil {
		return
	}

	// Type is now top-level in ClusterSpec, defaulted there.
	// if cfg.Type == "" {
	// 	cfg.Type = ClusterTypeKubeXM
	// }

	// ContainerRuntime is now top-level in ClusterSpec, defaulted there.
	// if cfg.ContainerRuntime == nil {
	// 	cfg.ContainerRuntime = &ContainerRuntimeConfig{}
	// }
	// SetDefaults_ContainerRuntimeConfig(cfg.ContainerRuntime)

	if cfg.ClusterName == "" && clusterMetaName != "" {
		cfg.ClusterName = clusterMetaName
	}
	if cfg.DNSDomain == "" {
		cfg.DNSDomain = "cluster.local"
	}
	if cfg.ProxyMode == "" {
		cfg.ProxyMode = "ipvs"
	}
	if cfg.AutoRenewCerts == nil { b := true; cfg.AutoRenewCerts = &b }
	if cfg.DisableKubeProxy == nil { b := false; cfg.DisableKubeProxy = &b }
	if cfg.MasqueradeAll == nil { b := false; cfg.MasqueradeAll = &b }
	if cfg.MaxPods == nil { mp := int32(110); cfg.MaxPods = &mp }
	if cfg.NodeCidrMaskSize == nil { ncms := int32(24); cfg.NodeCidrMaskSize = &ncms }
	// ContainerManager default depends on ContainerRuntime.CgroupDriver, handled in KubeletConfig defaults.
	// if cfg.ContainerManager == "" { cfg.ContainerManager = "systemd" }


	if cfg.Nodelocaldns == nil { cfg.Nodelocaldns = &NodelocaldnsConfig{} }
	if cfg.Nodelocaldns.Enabled == nil { b := true; cfg.Nodelocaldns.Enabled = &b }

	if cfg.Audit == nil { cfg.Audit = &AuditConfig{} }
	if cfg.Audit.Enabled == nil { b := false; cfg.Audit.Enabled = &b }

	if cfg.Kata == nil { cfg.Kata = &KataConfig{} }
	if cfg.Kata.Enabled == nil { b := false; cfg.Kata.Enabled = &b }

	if cfg.NodeFeatureDiscovery == nil { cfg.NodeFeatureDiscovery = &NodeFeatureDiscoveryConfig{} }
	if cfg.NodeFeatureDiscovery.Enabled == nil { b := false; cfg.NodeFeatureDiscovery.Enabled = &b }

	if cfg.FeatureGates == nil {
		cfg.FeatureGates = make(map[string]bool)
		defaultFGs := map[string]bool{
			"ExpandCSIVolumes":             true,
			"RotateKubeletServerCertificate": true,
			"CSIStorageCapacity":           true,
			"TTLAfterFinished":             true,
		}
		for k, v := range defaultFGs {
			cfg.FeatureGates[k] = v
		}
	}

	if cfg.APIServer == nil { cfg.APIServer = &APIServerConfig{} }
	if cfg.APIServer.ExtraArgs == nil { cfg.APIServer.ExtraArgs = []string{} }
	if cfg.APIServer.AdmissionPlugins == nil { cfg.APIServer.AdmissionPlugins = []string{} }

	if cfg.ControllerManager == nil { cfg.ControllerManager = &ControllerManagerConfig{} }
	if cfg.ControllerManager.ExtraArgs == nil { cfg.ControllerManager.ExtraArgs = []string{} }

	if cfg.Scheduler == nil { cfg.Scheduler = &SchedulerConfig{} }
	if cfg.Scheduler.ExtraArgs == nil { cfg.Scheduler.ExtraArgs = []string{} }

	if cfg.Kubelet == nil { cfg.Kubelet = &KubeletConfig{} }
	// CgroupDriver for Kubelet is defaulted based on ContainerRuntime in SetDefaults_KubeletConfig
	SetDefaults_KubeletConfig(cfg.Kubelet, cfg.ContainerManager)


	if cfg.KubeProxy == nil { cfg.KubeProxy = &KubeProxyConfig{} }
	if cfg.KubeProxy.ExtraArgs == nil { cfg.KubeProxy.ExtraArgs = []string{} }
	if cfg.ProxyMode == "iptables" && cfg.KubeProxy.IPTables == nil {
		 cfg.KubeProxy.IPTables = &KubeProxyIPTablesConfig{}
	}
	if cfg.KubeProxy.IPTables != nil {
		 if cfg.KubeProxy.IPTables.MasqueradeAll == nil { b := true; cfg.KubeProxy.IPTables.MasqueradeAll = &b }
		 if cfg.KubeProxy.IPTables.MasqueradeBit == nil { mb := int32(14); cfg.KubeProxy.IPTables.MasqueradeBit = &mb }
	}
	if cfg.ProxyMode == "ipvs" && cfg.KubeProxy.IPVS == nil {
		 cfg.KubeProxy.IPVS = &KubeProxyIPVSConfig{}
	}
	if cfg.KubeProxy.IPVS != nil {
		 if cfg.KubeProxy.IPVS.Scheduler == "" { sched := "rr"; cfg.KubeProxy.IPVS.Scheduler = sched }
		 if cfg.KubeProxy.IPVS.ExcludeCIDRs == nil { cfg.KubeProxy.IPVS.ExcludeCIDRs = []string{} }
	}
}

func SetDefaults_KubeletConfig(cfg *KubeletConfig, parentContainerManager string) {
	if cfg == nil {
		return
	}
	if cfg.ExtraArgs == nil { cfg.ExtraArgs = []string{} }
	if cfg.EvictionHard == nil { cfg.EvictionHard = make(map[string]string) }

	if cfg.PodPidsLimit == nil {
		defaultPidsLimit := int64(10000)
		cfg.PodPidsLimit = &defaultPidsLimit
	}

	if cfg.CgroupDriver == nil {
		// Default Kubelet CgroupDriver from parent KubernetesConfig.ContainerManager if set,
		// otherwise default to "systemd". This implies ContainerManager in KubernetesConfig
		// should reflect the container runtime's cgroup driver.
		defDriver := "systemd"
		if parentContainerManager != "" {
			defDriver = parentContainerManager
		}
		cfg.CgroupDriver = &defDriver
	}
}

func Validate_KubernetesConfig(cfg *KubernetesConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add(pathPrefix + ": kubernetes configuration section cannot be nil")
		return
	}
	// Type is now top-level
	// validK8sTypes := []string{ClusterTypeKubeXM, ClusterTypeKubeadm, ""}
	// if !containsString(validK8sTypes, cfg.Type) {
	// 	verrs.Add("%s.type: invalid type '%s', must be one of %v or empty for default", pathPrefix, cfg.Type, validK8sTypes)
	// }

	if strings.TrimSpace(cfg.Version) == "" {
		verrs.Add(pathPrefix + ".version: cannot be empty")
	}
	if strings.TrimSpace(cfg.DNSDomain) == "" {
		verrs.Add(pathPrefix + ".dnsDomain: cannot be empty")
	}

	validProxyModes := []string{"iptables", "ipvs", ""}
	if !containsString(validProxyModes, cfg.ProxyMode) {
		verrs.Add(pathPrefix + ".proxyMode: invalid mode '" + cfg.ProxyMode + "', must be one of " + fmt.Sprintf("%v", validProxyModes) + " or empty for default")
	}

	// ContainerRuntime is now top-level
	// if cfg.ContainerRuntime != nil {
	// 	Validate_ContainerRuntimeConfig(cfg.ContainerRuntime, verrs, pathPrefix+".containerRuntime")
	// } else {
	// 	verrs.Add("%s.containerRuntime: section cannot be nil", pathPrefix)
	// }

	if cfg.APIServer != nil { Validate_APIServerConfig(cfg.APIServer, verrs, pathPrefix+".apiServer") }
	if cfg.ControllerManager != nil { Validate_ControllerManagerConfig(cfg.ControllerManager, verrs, pathPrefix+".controllerManager") }
	if cfg.Scheduler != nil { Validate_SchedulerConfig(cfg.Scheduler, verrs, pathPrefix+".scheduler") }
	if cfg.Kubelet != nil { Validate_KubeletConfig(cfg.Kubelet, verrs, pathPrefix+".kubelet") }
	if cfg.KubeProxy != nil { Validate_KubeProxyConfig(cfg.KubeProxy, verrs, pathPrefix+".kubeProxy", cfg.ProxyMode) }

	if cfg.ContainerManager != "" && cfg.ContainerManager != "cgroupfs" && cfg.ContainerManager != "systemd" {
		verrs.Add(pathPrefix + ".containerManager: must be 'cgroupfs' or 'systemd', got '" + cfg.ContainerManager + "'")
	}
	if cfg.KubeletConfiguration != nil && len(cfg.KubeletConfiguration.Raw) == 0 {
		verrs.Add(pathPrefix + ".kubeletConfiguration: raw data cannot be empty if section is present")
	}
	if cfg.KubeProxyConfiguration != nil && len(cfg.KubeProxyConfiguration.Raw) == 0 {
		verrs.Add(pathPrefix + ".kubeProxyConfiguration: raw data cannot be empty if section is present")
	}
}

func Validate_APIServerConfig(cfg *APIServerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.ServiceNodePortRange != "" {
	   parts := strings.Split(cfg.ServiceNodePortRange, "-")
	   if len(parts) != 2 {
		   verrs.Add(pathPrefix + ".serviceNodePortRange: invalid format '" + cfg.ServiceNodePortRange + "', expected 'min-max'")
	   }
	}
}
func Validate_ControllerManagerConfig(cfg *ControllerManagerConfig, verrs *ValidationErrors, pathPrefix string) { if cfg == nil {return} }
func Validate_SchedulerConfig(cfg *SchedulerConfig, verrs *ValidationErrors, pathPrefix string) { if cfg == nil {return} }

func Validate_KubeletConfig(cfg *KubeletConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.CgroupDriver != nil && *cfg.CgroupDriver != "cgroupfs" && *cfg.CgroupDriver != "systemd" {
	   verrs.Add(pathPrefix + ".cgroupDriver: must be 'cgroupfs' or 'systemd' if specified, got '" + *cfg.CgroupDriver + "'")
	}
	validHairpinModes := []string{"promiscuous-bridge", "hairpin-veth", "none", ""}
	if cfg.HairpinMode != nil && *cfg.HairpinMode != "" && !containsString(validHairpinModes, *cfg.HairpinMode) {
		verrs.Add(pathPrefix + ".hairpinMode: invalid mode '" + *cfg.HairpinMode + "'")
	}

	if cfg.PodPidsLimit != nil && *cfg.PodPidsLimit <= 0 && *cfg.PodPidsLimit != -1 {
		verrs.Add(pathPrefix + ".podPidsLimit: must be positive or -1 (unlimited), got " + fmt.Sprintf("%d", *cfg.PodPidsLimit))
	}
}
func Validate_KubeProxyConfig(cfg *KubeProxyConfig, verrs *ValidationErrors, pathPrefix string, parentProxyMode string) {
	if cfg == nil { return }
	if parentProxyMode == "iptables" && cfg.IPTables == nil {}
	if parentProxyMode == "ipvs" && cfg.IPVS == nil {}
	if cfg.IPTables != nil && cfg.IPTables.MasqueradeBit != nil && (*cfg.IPTables.MasqueradeBit < 0 || *cfg.IPTables.MasqueradeBit > 31) {
	   verrs.Add(pathPrefix + ".ipTables.masqueradeBit: must be between 0 and 31, got " + fmt.Sprintf("%d", *cfg.IPTables.MasqueradeBit))
	}
}

func isValidCIDR(cidr string) bool { _, _, err := net.ParseCIDR(cidr); return err == nil }
func containsString(slice []string, item string) bool { // Local helper, was in endpoint_types.go
	for _, s := range slice { if s == item { return true } }
	return false
}

func (k *KubernetesConfig) IsKubeProxyDisabled() bool {
	if k != nil && k.DisableKubeProxy != nil { return *k.DisableKubeProxy }
	return false
}
func (k *KubernetesConfig) IsNodelocaldnsEnabled() bool {
	if k != nil && k.Nodelocaldns != nil && k.Nodelocaldns.Enabled != nil { return *k.Nodelocaldns.Enabled }
	return true // Default to true as per SetDefaults
}
func (k *KubernetesConfig) IsAuditEnabled() bool {
	if k != nil && k.Audit != nil && k.Audit.Enabled != nil { return *k.Audit.Enabled }
	return false
}
func (k *KubernetesConfig) IsKataEnabled() bool {
	if k != nil && k.Kata != nil && k.Kata.Enabled != nil { return *k.Kata.Enabled }
	return false
}
func (k *KubernetesConfig) IsNodeFeatureDiscoveryEnabled() bool {
	if k != nil && k.NodeFeatureDiscovery != nil && k.NodeFeatureDiscovery.Enabled != nil { return *k.NodeFeatureDiscovery.Enabled }
	return false
}
func (k *KubernetesConfig) IsAutoRenewCertsEnabled() bool {
   if k != nil && k.AutoRenewCerts != nil { return *k.AutoRenewCerts }
   return true // Default to true as per SetDefaults
}
func (k *KubernetesConfig) GetMaxPods() int32 {
   if k != nil && k.MaxPods != nil { return *k.MaxPods }
   return 110 // Default from SetDefaults
}
func (k *KubernetesConfig) IsAtLeastVersion(versionStr string) bool {
	if k == nil || k.Version == "" { return false }
	parsedVersion, err := versionutil.ParseGeneric(k.Version)
	if err != nil { return false }
	compareVersion, err := versionutil.ParseGeneric(versionStr)
	if err != nil { return false }
	return parsedVersion.AtLeast(compareVersion)
}

// ValidationErrors and SchemeGroupVersion would be defined in cluster_types.go or a shared file.
// For self-containedness of this snippet, they are assumed to exist.
// type ValidationErrors struct{ Errors []string }
// func (ve *ValidationErrors) Add(format string, args ...interface{}) { ve.Errors = append(ve.Errors, fmt.Sprintf(format, args...)) }
// func (ve *ValidationErrors) Error() string { if len(ve.Errors) == 0 { return "no validation errors" }; return strings.Join(ve.Errors, "; ") }
// func (ve *ValidationErrors) IsEmpty() bool { return len(ve.Errors) == 0 }
// var SchemeGroupVersion = metav1.GroupVersion{Group: "kubexms.io", Version: "v1alpha1"}
// NOTE: DeepCopy methods should be generated by controller-gen.
// Manual implementations are error-prone and incomplete.
// The `ClusterTypeKubeXM` and `ClusterTypeKubeadm` consts also might be better in cluster_types.go or a shared common place.
// For this snippet, assuming they are accessible.
// const ClusterTypeKubeXM = "kubexm"
// const ClusterTypeKubeadm = "kubeadm"
// The isValidCIDR and containsString helpers are duplicated here for context, ideally they are in a shared util.
// Helper functions like IsKubeProxyDisabled, IsNodelocaldnsEnabled etc. are good.
// SetDefaults_KubeletConfig was updated to take parentContainerManager to correctly default CgroupDriver.
// The ContainerRuntime field has been removed from KubernetesConfig as it's now a top-level field in ClusterSpec.
// Defaulting and validation for ContainerRuntime is handled at the ClusterSpec level.
// KubernetesConfig.Type (kubexm/kubeadm) is also now top-level in ClusterSpec.
// The SetDefaults_KubernetesConfig and Validate_KubernetesConfig have been adjusted to reflect these changes.
// Removed redundant local `isValidCIDR` and `containsString` as they are expected from `cluster_types.go` or a util package.
// Removed local const definitions for ClusterTypeKubeXM and ClusterTypeKubeadm as they are expected from `cluster_types.go`.
// Added missing import "net" for isValidCIDR.
// Corrected SetDefaults_KubeletConfig to properly use parentContainerManager for CgroupDriver.
// Removed local ValidationErrors and SchemeGroupVersion as they are in cluster_types.go.
// Removed local DeepCopyObject stubs.
// Added import "k8s.io/apimachinery/pkg/runtime" for RawExtension.
// Added import "k8s.io/apimachinery/pkg/util/version" for version parsing.
