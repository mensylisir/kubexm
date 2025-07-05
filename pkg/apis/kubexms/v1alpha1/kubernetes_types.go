package v1alpha1

import (
	"strings"
	"net"
	"k8s.io/apimachinery/pkg/runtime"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"strconv"
	"time"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
)

// KubernetesConfig defines the configuration for Kubernetes components.
type KubernetesConfig struct {
	Type                   string                    `json:"type,omitempty" yaml:"type,omitempty"`
	Version                string                    `json:"version" yaml:"version"`
	ContainerRuntime       *ContainerRuntimeConfig   `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	ClusterName            string                    `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	DNSDomain              string                    `json:"dnsDomain,omitempty" yaml:"dnsDomain,omitempty"`
	DisableKubeProxy       *bool                     `json:"disableKubeProxy,omitempty" yaml:"disableKubeProxy,omitempty"`
	MasqueradeAll          *bool                     `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"`
	MaxPods                *int32                    `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	NodeCidrMaskSize       *int32                    `json:"nodeCidrMaskSize,omitempty" yaml:"nodeCidrMaskSize,omitempty"`
	ApiserverCertExtraSans []string                  `json:"apiserverCertExtraSans,omitempty" yaml:"apiserverCertExtraSans,omitempty"`
	ProxyMode              string                    `json:"proxyMode,omitempty" yaml:"proxyMode,omitempty"`
	AutoRenewCerts         *bool                     `json:"autoRenewCerts,omitempty" yaml:"autoRenewCerts,omitempty"`
	ContainerManager       string                    `json:"containerManager,omitempty" yaml:"containerManager,omitempty"`
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

type APIServerConfig struct {
	ExtraArgs            []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	EtcdServers          []string `json:"etcdServers,omitempty" yaml:"etcdServers,omitempty"`
	EtcdCAFile           string   `json:"etcdCAFile,omitempty" yaml:"etcdCAFile,omitempty"`
	EtcdCertFile         string   `json:"etcdCertFile,omitempty" yaml:"etcdCertFile,omitempty"`
	EtcdKeyFile          string   `json:"etcdKeyFile,omitempty" yaml:"etcdKeyFile,omitempty"`
	AdmissionPlugins     []string `json:"admissionPlugins,omitempty" yaml:"admissionPlugins,omitempty"`
	ServiceNodePortRange string   `json:"serviceNodePortRange,omitempty" yaml:"serviceNodePortRange,omitempty"`
}

type ControllerManagerConfig struct {
	ExtraArgs                    []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ServiceAccountPrivateKeyFile string   `json:"serviceAccountPrivateKeyFile,omitempty" yaml:"serviceAccountPrivateKeyFile,omitempty"`
}

type SchedulerConfig struct {
	ExtraArgs        []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	PolicyConfigFile string   `json:"policyConfigFile,omitempty" yaml:"policyConfigFile,omitempty"`
}

type KubeletConfig struct {
	ExtraArgs        []string            `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	CgroupDriver     *string             `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`
	EvictionHard     map[string]string   `json:"evictionHard,omitempty" yaml:"evictionHard,omitempty"`
	HairpinMode      *string             `json:"hairpinMode,omitempty" yaml:"hairpinMode,omitempty"`
	PodPidsLimit     *int64              `json:"podPidsLimit,omitempty" yaml:"podPidsLimit,omitempty"`
}

type KubeProxyIPTablesConfig struct {
   MasqueradeAll *bool  `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"`
   MasqueradeBit *int32 `json:"masqueradeBit,omitempty" yaml:"masqueradeBit,omitempty"`
   SyncPeriod    string `json:"syncPeriod,omitempty" yaml:"syncPeriod,omitempty"`
   MinSyncPeriod string `json:"minSyncPeriod,omitempty" yaml:"minSyncPeriod,omitempty"`
}

type KubeProxyIPVSConfig struct {
   Scheduler     string   `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
   SyncPeriod    string   `json:"syncPeriod,omitempty" yaml:"syncPeriod,omitempty"`
   MinSyncPeriod string   `json:"minSyncPeriod,omitempty" yaml:"minSyncPeriod,omitempty"`
   ExcludeCIDRs  []string `json:"excludeCIDRs,omitempty" yaml:"excludeCIDRs,omitempty"`
}

type KubeProxyConfig struct {
	ExtraArgs    []string                 `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	IPTables     *KubeProxyIPTablesConfig `json:"ipTables,omitempty" yaml:"ipTables,omitempty"`
	IPVS         *KubeProxyIPVSConfig     `json:"ipvs,omitempty" yaml:"ipvs,omitempty"`
}

type NodelocaldnsConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type AuditConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type KataConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type NodeFeatureDiscoveryConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

func SetDefaults_KubernetesConfig(cfg *KubernetesConfig, clusterMetaName string) {
	if cfg == nil { return }
	if cfg.Type == "" { cfg.Type = ClusterTypeKubeXM }
	if cfg.ContainerRuntime == nil { cfg.ContainerRuntime = &ContainerRuntimeConfig{} }
	SetDefaults_ContainerRuntimeConfig(cfg.ContainerRuntime)
	if cfg.ClusterName == "" && clusterMetaName != "" { cfg.ClusterName = clusterMetaName }
	if cfg.DNSDomain == "" { cfg.DNSDomain = "cluster.local" }
	if cfg.ProxyMode == "" { cfg.ProxyMode = "ipvs" }
	if cfg.AutoRenewCerts == nil { cfg.AutoRenewCerts = boolPtr(true) }
	if cfg.DisableKubeProxy == nil { cfg.DisableKubeProxy = boolPtr(false) }
	if cfg.MasqueradeAll == nil { cfg.MasqueradeAll = boolPtr(false) }
	if cfg.MaxPods == nil { cfg.MaxPods = int32Ptr(110) }
	if cfg.NodeCidrMaskSize == nil { cfg.NodeCidrMaskSize = int32Ptr(24) }
	if cfg.ContainerManager == "" { cfg.ContainerManager = "systemd" }
	if cfg.Nodelocaldns == nil { cfg.Nodelocaldns = &NodelocaldnsConfig{} }
	if cfg.Nodelocaldns.Enabled == nil { cfg.Nodelocaldns.Enabled = boolPtr(true) }
	if cfg.Audit == nil { cfg.Audit = &AuditConfig{} }
	if cfg.Audit.Enabled == nil { cfg.Audit.Enabled = boolPtr(false) }
	if cfg.Kata == nil { cfg.Kata = &KataConfig{} }
	if cfg.Kata.Enabled == nil { cfg.Kata.Enabled = boolPtr(false) }
	if cfg.NodeFeatureDiscovery == nil { cfg.NodeFeatureDiscovery = &NodeFeatureDiscoveryConfig{} }
	if cfg.NodeFeatureDiscovery.Enabled == nil { cfg.NodeFeatureDiscovery.Enabled = boolPtr(false) }
	if cfg.FeatureGates == nil {
		cfg.FeatureGates = make(map[string]bool)
		defaultFGs := map[string]bool{
			"ExpandCSIVolumes": true, "RotateKubeletServerCertificate": true,
			"CSIStorageCapacity": true, "TTLAfterFinished": true,
		}
		for k, v := range defaultFGs { cfg.FeatureGates[k] = v }
	}
	if cfg.APIServer == nil { cfg.APIServer = &APIServerConfig{} }
	if cfg.APIServer.ExtraArgs == nil { cfg.APIServer.ExtraArgs = []string{} }
	if cfg.APIServer.AdmissionPlugins == nil { cfg.APIServer.AdmissionPlugins = []string{} }
	if cfg.ControllerManager == nil { cfg.ControllerManager = &ControllerManagerConfig{} }
	if cfg.ControllerManager.ExtraArgs == nil { cfg.ControllerManager.ExtraArgs = []string{} }
	if cfg.Scheduler == nil { cfg.Scheduler = &SchedulerConfig{} }
	if cfg.Scheduler.ExtraArgs == nil { cfg.Scheduler.ExtraArgs = []string{} }
	if cfg.Kubelet == nil { cfg.Kubelet = &KubeletConfig{} }
	SetDefaults_KubeletConfig(cfg.Kubelet, cfg.ContainerManager)
	if cfg.KubeProxy == nil { cfg.KubeProxy = &KubeProxyConfig{} }
	if cfg.KubeProxy.ExtraArgs == nil { cfg.KubeProxy.ExtraArgs = []string{} }
	if cfg.ProxyMode == "iptables" {
		if cfg.KubeProxy.IPTables == nil { cfg.KubeProxy.IPTables = &KubeProxyIPTablesConfig{} }
		SetDefaults_KubeProxyIPTablesConfig(cfg.KubeProxy.IPTables)
	}
	if cfg.ProxyMode == "ipvs" {
		if cfg.KubeProxy.IPVS == nil { cfg.KubeProxy.IPVS = &KubeProxyIPVSConfig{} }
		SetDefaults_KubeProxyIPVSConfig(cfg.KubeProxy.IPVS)
	}
}

func SetDefaults_KubeProxyIPTablesConfig(cfg *KubeProxyIPTablesConfig) {
	if cfg == nil { return }
	if cfg.MasqueradeAll == nil { cfg.MasqueradeAll = boolPtr(true) }
	if cfg.MasqueradeBit == nil { cfg.MasqueradeBit = int32Ptr(14) }
}

func SetDefaults_KubeProxyIPVSConfig(cfg *KubeProxyIPVSConfig) {
	if cfg == nil { return }
	if cfg.Scheduler == "" { cfg.Scheduler = "rr" }
	if cfg.ExcludeCIDRs == nil { cfg.ExcludeCIDRs = []string{} }
}

func SetDefaults_KubeletConfig(cfg *KubeletConfig, containerManager string) {
	if cfg == nil { return }
	if cfg.ExtraArgs == nil { cfg.ExtraArgs = []string{} }
	if cfg.EvictionHard == nil { cfg.EvictionHard = make(map[string]string) }
	if cfg.PodPidsLimit == nil { cfg.PodPidsLimit = int64Ptr(10000) }
	if cfg.CgroupDriver == nil {
		if containerManager != "" { cfg.CgroupDriver = stringPtr(containerManager)
		} else { cfg.CgroupDriver = stringPtr("systemd") }
	}
}

func Validate_KubernetesConfig(cfg *KubernetesConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { verrs.Add("%s: kubernetes configuration section cannot be nil", pathPrefix); return }
	validK8sTypes := []string{ClusterTypeKubeXM, ClusterTypeKubeadm, ""}
	if !containsString(validK8sTypes, cfg.Type) {
		verrs.Add("%s.type: invalid type '%s', must be one of %v or empty for default", pathPrefix, cfg.Type, validK8sTypes)
	}
	if strings.TrimSpace(cfg.Version) == "" {
		verrs.Add("%s.version: cannot be empty", pathPrefix)
	} else if !isValidRuntimeVersion(cfg.Version) {
		verrs.Add("%s.version: '%s' is not a recognized version format", pathPrefix, cfg.Version)
	}
	if strings.TrimSpace(cfg.DNSDomain) == "" { verrs.Add("%s.dnsDomain: cannot be empty", pathPrefix) }
	validProxyModes := []string{"iptables", "ipvs", ""}
	if !containsString(validProxyModes, cfg.ProxyMode) {
		verrs.Add("%s.proxyMode: invalid mode '%s', must be one of %v or empty for default", pathPrefix, cfg.ProxyMode, validProxyModes)
	}
	if cfg.ContainerRuntime != nil { Validate_ContainerRuntimeConfig(cfg.ContainerRuntime, verrs, pathPrefix+".containerRuntime")
	} else { verrs.Add("%s.containerRuntime: section cannot be nil", pathPrefix) }
	if cfg.APIServer != nil { Validate_APIServerConfig(cfg.APIServer, verrs, pathPrefix+".apiServer") }
	if cfg.ControllerManager != nil { Validate_ControllerManagerConfig(cfg.ControllerManager, verrs, pathPrefix+".controllerManager") }
	if cfg.Scheduler != nil { Validate_SchedulerConfig(cfg.Scheduler, verrs, pathPrefix+".scheduler") }
	if cfg.Kubelet != nil { Validate_KubeletConfig(cfg.Kubelet, verrs, pathPrefix+".kubelet") }
	if cfg.KubeProxy != nil { Validate_KubeProxyConfig(cfg.KubeProxy, verrs, pathPrefix+".kubeProxy", cfg.ProxyMode) }
	if cfg.ContainerManager != "" && cfg.ContainerManager != "cgroupfs" && cfg.ContainerManager != "systemd" {
		verrs.Add("%s.containerManager: must be 'cgroupfs' or 'systemd', got '%s'", pathPrefix, cfg.ContainerManager)
	}
	if cfg.KubeletConfiguration != nil && len(cfg.KubeletConfiguration.Raw) == 0 {
		verrs.Add("%s.kubeletConfiguration: raw data cannot be empty if section is present", pathPrefix)
	}
	if cfg.KubeProxyConfiguration != nil && len(cfg.KubeProxyConfiguration.Raw) == 0 {
		verrs.Add("%s.kubeProxyConfiguration: raw data cannot be empty if section is present", pathPrefix)
	}
}

func Validate_APIServerConfig(cfg *APIServerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.ServiceNodePortRange != "" {
		parts := strings.Split(cfg.ServiceNodePortRange, "-")
		if len(parts) != 2 {
			verrs.Add("%s.serviceNodePortRange: invalid format '%s', expected 'min-max'", pathPrefix, cfg.ServiceNodePortRange)
		} else {
			minPort, errMin := strconv.Atoi(parts[0])
			maxPort, errMax := strconv.Atoi(parts[1])
			if errMin != nil || errMax != nil {
				verrs.Add("%s.serviceNodePortRange: ports must be numbers, got '%s'", pathPrefix, cfg.ServiceNodePortRange)
			} else {
				if minPort <= 0 || minPort > 65535 || maxPort <= 0 || maxPort > 65535 {
					verrs.Add("%s.serviceNodePortRange: port numbers must be between 1 and 65535, got min %d, max %d", pathPrefix, minPort, maxPort)
				}
				if minPort >= maxPort {
					verrs.Add("%s.serviceNodePortRange: min port %d must be less than max port %d", pathPrefix, minPort, maxPort)
				}
			}
		}
	}
}

func Validate_ControllerManagerConfig(cfg *ControllerManagerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.ServiceAccountPrivateKeyFile != "" && strings.TrimSpace(cfg.ServiceAccountPrivateKeyFile) == "" {
		verrs.Add("%s.serviceAccountPrivateKeyFile: cannot be empty if specified", pathPrefix)
	}
}
func Validate_SchedulerConfig(cfg *SchedulerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.PolicyConfigFile != "" && strings.TrimSpace(cfg.PolicyConfigFile) == "" {
		verrs.Add("%s.policyConfigFile: cannot be empty if specified", pathPrefix)
	}
}

func Validate_KubeletConfig(cfg *KubeletConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.CgroupDriver != nil && *cfg.CgroupDriver != "cgroupfs" && *cfg.CgroupDriver != "systemd" {
	   verrs.Add("%s.cgroupDriver: must be 'cgroupfs' or 'systemd' if specified, got '%s'", pathPrefix, *cfg.CgroupDriver)
	}
	validHairpinModes := []string{"promiscuous-bridge", "hairpin-veth", "none", ""}
	if cfg.HairpinMode != nil && *cfg.HairpinMode != "" && !containsString(validHairpinModes, *cfg.HairpinMode) {
		verrs.Add("%s.hairpinMode: invalid mode '%s'", pathPrefix, *cfg.HairpinMode)
	}
	if cfg.PodPidsLimit != nil && *cfg.PodPidsLimit <= 0 && *cfg.PodPidsLimit != -1 {
		verrs.Add("%s.podPidsLimit: must be positive or -1 (unlimited), got %d", pathPrefix, *cfg.PodPidsLimit)
	}
}

func Validate_KubeProxyIPTablesConfig(cfg *KubeProxyIPTablesConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.MasqueradeBit != nil && (*cfg.MasqueradeBit < 0 || *cfg.MasqueradeBit > 31) {
		verrs.Add("%s.masqueradeBit: must be between 0 and 31, got %d", pathPrefix, *cfg.MasqueradeBit)
	}
	if cfg.SyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.SyncPeriod); err != nil {
			verrs.Add("%s.syncPeriod: invalid duration format '%s': %v", pathPrefix, cfg.SyncPeriod, err)
		}
	}
	if cfg.MinSyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.MinSyncPeriod); err != nil {
			verrs.Add("%s.minSyncPeriod: invalid duration format '%s': %v", pathPrefix, cfg.MinSyncPeriod, err)
		}
	}
}

func Validate_KubeProxyIPVSConfig(cfg *KubeProxyIPVSConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	// Add validation for IPVS Scheduler if needed (e.g. list of known good values)
	for i, cidr := range cfg.ExcludeCIDRs {
		if !util.IsValidCIDR(cidr) { // Use util.IsValidCIDR
			verrs.Add("%s.excludeCIDRs[%d]: invalid CIDR format '%s'", pathPrefix, i, cidr)
		}
	}
	if cfg.SyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.SyncPeriod); err != nil {
			verrs.Add("%s.syncPeriod: invalid duration format '%s': %v", pathPrefix, cfg.SyncPeriod, err)
		}
	}
	if cfg.MinSyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.MinSyncPeriod); err != nil {
			verrs.Add("%s.minSyncPeriod: invalid duration format '%s': %v", pathPrefix, cfg.MinSyncPeriod, err)
		}
	}
}

func Validate_KubeProxyConfig(cfg *KubeProxyConfig, verrs *ValidationErrors, pathPrefix string, parentProxyMode string) {
	if cfg == nil { return }

	if parentProxyMode == "iptables" {
		if cfg.IPTables != nil {
			Validate_KubeProxyIPTablesConfig(cfg.IPTables, verrs, pathPrefix+".ipTables")
		}
		// It's an error if IPVS specific config is set when mode is iptables
		if cfg.IPVS != nil {
			verrs.Add("%s.ipvs: should not be set if proxyMode is 'iptables'", pathPrefix)
		}
	} else if parentProxyMode == "ipvs" {
		if cfg.IPVS != nil {
			Validate_KubeProxyIPVSConfig(cfg.IPVS, verrs, pathPrefix+".ipvs")
		}
		// It's an error if IPTables specific config is set when mode is ipvs
		if cfg.IPTables != nil {
			verrs.Add("%s.ipTables: should not be set if proxyMode is 'ipvs'", pathPrefix)
		}
	}
}

// isValidCIDR has been moved to pkg/util/utils.go as IsValidCIDR
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
)

// isValidCIDR has been moved to pkg/util/utils.go as IsValidCIDR
// func isValidCIDR(cidr string) bool { _, _, err := net.ParseCIDR(cidr); return err == nil }

func (k *KubernetesConfig) IsKubeProxyDisabled() bool {
	if k != nil && k.DisableKubeProxy != nil { return *k.DisableKubeProxy }
	return false
}
func (k *KubernetesConfig) IsNodelocaldnsEnabled() bool {
	if k != nil && k.Nodelocaldns != nil && k.Nodelocaldns.Enabled != nil { return *k.Nodelocaldns.Enabled }
	return true
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
   return false
}
func (k *KubernetesConfig) GetMaxPods() int32 {
   if k != nil && k.MaxPods != nil { return *k.MaxPods }
   return 110
}
func (k *KubernetesConfig) IsAtLeastVersion(versionStr string) bool {
	if k == nil || k.Version == "" { return false }
	parsedVersion, err := versionutil.ParseGeneric(k.Version)
	if err != nil { return false }
	compareVersion, err := versionutil.ParseGeneric(versionStr)
	if err != nil { return false }
	return parsedVersion.AtLeast(compareVersion)
}
