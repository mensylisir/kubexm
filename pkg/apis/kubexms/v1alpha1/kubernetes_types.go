package v1alpha1

import (
	"strings"
	// "net" // No longer needed directly as isValidCIDR moved to util
	"k8s.io/apimachinery/pkg/runtime"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"fmt"
	"strconv"
	"time"

	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
	"github.com/mensylisir/kubexm/pkg/common" // Import common for constants
	"github.com/mensylisir/kubexm/pkg/util/validation"
)


var (
	// validK8sTypes lists the supported Kubernetes deployment types by this configuration.
	validK8sTypes = []string{common.ClusterTypeKubeXM, common.ClusterTypeKubeadm, ""} // Empty string allows for default
	// validProxyModes lists the supported KubeProxy modes.
	// Using constants from pkg/common
	validProxyModes = []string{common.KubeProxyModeIPTables, common.KubeProxyModeIPVS, ""} // Empty string allows for default
	// validKubeletCgroupDrivers lists the supported cgroup drivers for Kubelet.
	// Using constants from pkg/common
	validKubeletCgroupDrivers = []string{common.CgroupDriverSystemd, common.CgroupDriverCgroupfs}
	// validKubeletHairpinModes lists the supported hairpin modes for Kubelet.
	validKubeletHairpinModes = []string{"promiscuous-bridge", "hairpin-veth", "none", ""} // Empty string allows for default
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

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodelocaldnsConfig) DeepCopyInto(out *NodelocaldnsConfig) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodelocaldnsConfig.
func (in *NodelocaldnsConfig) DeepCopy() *NodelocaldnsConfig {
	if in == nil {
		return nil
	}
	out := new(NodelocaldnsConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditConfig) DeepCopyInto(out *AuditConfig) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditConfig.
func (in *AuditConfig) DeepCopy() *AuditConfig {
	if in == nil {
		return nil
	}
	out := new(AuditConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KataConfig) DeepCopyInto(out *KataConfig) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KataConfig.
func (in *KataConfig) DeepCopy() *KataConfig {
	if in == nil {
		return nil
	}
	out := new(KataConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeFeatureDiscoveryConfig) DeepCopyInto(out *NodeFeatureDiscoveryConfig) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeFeatureDiscoveryConfig.
func (in *NodeFeatureDiscoveryConfig) DeepCopy() *NodeFeatureDiscoveryConfig {
	if in == nil {
		return nil
	}
	out := new(NodeFeatureDiscoveryConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeProxyIPTablesConfig) DeepCopyInto(out *KubeProxyIPTablesConfig) {
	*out = *in
	if in.MasqueradeAll != nil {
		in, out := &in.MasqueradeAll, &out.MasqueradeAll
		*out = new(bool)
		**out = *in
	}
	if in.MasqueradeBit != nil {
		in, out := &in.MasqueradeBit, &out.MasqueradeBit
		*out = new(int32)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeProxyIPTablesConfig.
func (in *KubeProxyIPTablesConfig) DeepCopy() *KubeProxyIPTablesConfig {
	if in == nil {
		return nil
	}
	out := new(KubeProxyIPTablesConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeProxyIPVSConfig) DeepCopyInto(out *KubeProxyIPVSConfig) {
	*out = *in
	if in.ExcludeCIDRs != nil {
		in, out := &in.ExcludeCIDRs, &out.ExcludeCIDRs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeProxyIPVSConfig.
func (in *KubeProxyIPVSConfig) DeepCopy() *KubeProxyIPVSConfig {
	if in == nil {
		return nil
	}
	out := new(KubeProxyIPVSConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerConfig) DeepCopyInto(out *APIServerConfig) {
	*out = *in
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EtcdServers != nil {
		in, out := &in.EtcdServers, &out.EtcdServers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AdmissionPlugins != nil {
		in, out := &in.AdmissionPlugins, &out.AdmissionPlugins
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerConfig.
func (in *APIServerConfig) DeepCopy() *APIServerConfig {
	if in == nil {
		return nil
	}
	out := new(APIServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerManagerConfig) DeepCopyInto(out *ControllerManagerConfig) {
	*out = *in
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerManagerConfig.
func (in *ControllerManagerConfig) DeepCopy() *ControllerManagerConfig {
	if in == nil {
		return nil
	}
	out := new(ControllerManagerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchedulerConfig) DeepCopyInto(out *SchedulerConfig) {
	*out = *in
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchedulerConfig.
func (in *SchedulerConfig) DeepCopy() *SchedulerConfig {
	if in == nil {
		return nil
	}
	out := new(SchedulerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeletConfig) DeepCopyInto(out *KubeletConfig) {
	*out = *in
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.CgroupDriver != nil {
		in, out := &in.CgroupDriver, &out.CgroupDriver
		*out = new(string)
		**out = *in
	}
	if in.EvictionHard != nil {
		in, out := &in.EvictionHard, &out.EvictionHard
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.HairpinMode != nil {
		in, out := &in.HairpinMode, &out.HairpinMode
		*out = new(string)
		**out = *in
	}
	if in.PodPidsLimit != nil {
		in, out := &in.PodPidsLimit, &out.PodPidsLimit
		*out = new(int64)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeletConfig.
func (in *KubeletConfig) DeepCopy() *KubeletConfig {
	if in == nil {
		return nil
	}
	out := new(KubeletConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeProxyConfig) DeepCopyInto(out *KubeProxyConfig) {
	*out = *in
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.IPTables != nil {
		in, out := &in.IPTables, &out.IPTables
		*out = new(KubeProxyIPTablesConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.IPVS != nil {
		in, out := &in.IPVS, &out.IPVS
		*out = new(KubeProxyIPVSConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeProxyConfig.
func (in *KubeProxyConfig) DeepCopy() *KubeProxyConfig {
	if in == nil {
		return nil
	}
	out := new(KubeProxyConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubernetesConfig) DeepCopyInto(out *KubernetesConfig) {
	*out = *in
	if in.ContainerRuntime != nil {
		in, out := &in.ContainerRuntime, &out.ContainerRuntime
		*out = new(ContainerRuntimeConfig)
		(*in).DeepCopyInto(*out) // Assumes ContainerRuntimeConfig has DeepCopyInto
	}
	if in.ApiserverCertExtraSans != nil {
		in, out := &in.ApiserverCertExtraSans, &out.ApiserverCertExtraSans
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.FeatureGates != nil {
		in, out := &in.FeatureGates, &out.FeatureGates
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.APIServer != nil {
		in, out := &in.APIServer, &out.APIServer
		*out = new(APIServerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ControllerManager != nil {
		in, out := &in.ControllerManager, &out.ControllerManager
		*out = new(ControllerManagerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Scheduler != nil {
		in, out := &in.Scheduler, &out.Scheduler
		*out = new(SchedulerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Kubelet != nil {
		in, out := &in.Kubelet, &out.Kubelet
		*out = new(KubeletConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.KubeProxy != nil {
		in, out := &in.KubeProxy, &out.KubeProxy
		*out = new(KubeProxyConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.KubeletConfiguration != nil {
		in, out := &in.KubeletConfiguration, &out.KubeletConfiguration
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	if in.KubeProxyConfiguration != nil {
		in, out := &in.KubeProxyConfiguration, &out.KubeProxyConfiguration
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	if in.Nodelocaldns != nil {
		in, out := &in.Nodelocaldns, &out.Nodelocaldns
		*out = new(NodelocaldnsConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Audit != nil {
		in, out := &in.Audit, &out.Audit
		*out = new(AuditConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Kata != nil {
		in, out := &in.Kata, &out.Kata
		*out = new(KataConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeFeatureDiscovery != nil {
		in, out := &in.NodeFeatureDiscovery, &out.NodeFeatureDiscovery
		*out = new(NodeFeatureDiscoveryConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubernetesConfig.
func (in *KubernetesConfig) DeepCopy() *KubernetesConfig {
	if in == nil {
		return nil
	}
	out := new(KubernetesConfig)
	in.DeepCopyInto(out)
	return out
}

func SetDefaults_KubernetesConfig(cfg *KubernetesConfig, clusterMetaName string) {
	if cfg == nil { return }
	if cfg.Type == "" { cfg.Type = common.ClusterTypeKubeXM }
	if cfg.ContainerRuntime == nil { cfg.ContainerRuntime = &ContainerRuntimeConfig{} }
	SetDefaults_ContainerRuntimeConfig(cfg.ContainerRuntime)
	if cfg.ClusterName == "" && clusterMetaName != "" { cfg.ClusterName = clusterMetaName }
	if cfg.DNSDomain == "" { cfg.DNSDomain = "cluster.local" }
	if cfg.ProxyMode == "" { cfg.ProxyMode = common.KubeProxyModeIPVS } // Use common constant
	if cfg.AutoRenewCerts == nil { cfg.AutoRenewCerts = util.BoolPtr(true) }
	if cfg.DisableKubeProxy == nil { cfg.DisableKubeProxy = util.BoolPtr(false) }
	if cfg.MasqueradeAll == nil { cfg.MasqueradeAll = util.BoolPtr(false) }
	if cfg.MaxPods == nil { cfg.MaxPods = util.Int32Ptr(110) }
	if cfg.NodeCidrMaskSize == nil { cfg.NodeCidrMaskSize = util.Int32Ptr(24) }
	if cfg.ContainerManager == "" { cfg.ContainerManager = common.CgroupDriverSystemd } // Use common constant
	if cfg.Nodelocaldns == nil { cfg.Nodelocaldns = &NodelocaldnsConfig{} }
	if cfg.Nodelocaldns.Enabled == nil { cfg.Nodelocaldns.Enabled = util.BoolPtr(true) }
	if cfg.Audit == nil { cfg.Audit = &AuditConfig{} }
	if cfg.Audit.Enabled == nil { cfg.Audit.Enabled = util.BoolPtr(false) }
	if cfg.Kata == nil { cfg.Kata = &KataConfig{} }
	if cfg.Kata.Enabled == nil { cfg.Kata.Enabled = util.BoolPtr(false) }
	if cfg.NodeFeatureDiscovery == nil { cfg.NodeFeatureDiscovery = &NodeFeatureDiscoveryConfig{} }
	if cfg.NodeFeatureDiscovery.Enabled == nil { cfg.NodeFeatureDiscovery.Enabled = util.BoolPtr(false) }
	if cfg.FeatureGates == nil {
		cfg.FeatureGates = make(map[string]bool)
		defaultFGs := map[string]bool{
			"ExpandCSIVolumes": true, "RotateKubeletServerCertificate": true,
			"CSIStorageCapacity": true, "TTLAfterFinished": true,
		}
		for k, v := range defaultFGs { cfg.FeatureGates[k] = v }
	}
	if cfg.APIServer == nil { cfg.APIServer = &APIServerConfig{} }
	// Ensure ExtraArgs is initialized before merging
	if cfg.APIServer.ExtraArgs == nil { cfg.APIServer.ExtraArgs = []string{} }
	defaultAPIServerArgs := map[string]string{
		"--profiling":                     "--profiling=false",
		"--anonymous-auth":                "--anonymous-auth=false",
		"--service-account-lookup":        "--service-account-lookup=true",
		"--audit-log-path":                "--audit-log-path=/var/log/kubernetes/apiserver-audit.log",
		"--audit-log-maxage":              "--audit-log-maxage=30",
		"--audit-log-maxbackup":           "--audit-log-maxbackup=10",
		"--audit-log-maxsize":             "--audit-log-maxsize=100",
		"--authorization-mode":            "--authorization-mode=Node,RBAC", // Will be merged smartly by EnsureExtraArgs if Node or RBAC already present
		"--tls-cipher-suites":             "--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"--request-timeout":               "--request-timeout=3m0s",
		// --kubelet-client-certificate, --kubelet-client-key, --service-account-signing-key-file, --service-account-issuer, --encryption-provider-config
		// are highly dependent on runtime generation and paths, so not defaulted here.
	}
	cfg.APIServer.ExtraArgs = util.EnsureExtraArgs(cfg.APIServer.ExtraArgs, defaultAPIServerArgs)

	if cfg.APIServer.AdmissionPlugins == nil {
		cfg.APIServer.AdmissionPlugins = []string{} // Initialize if nil
	}
	// Default admission plugins - ensure core plugins are present without overriding user's complete custom list if provided non-empty
	defaultAdmissionPlugins := []string{
		"NodeRestriction", "NamespaceLifecycle", "LimitRanger", "ServiceAccount",
		"DefaultStorageClass", "DefaultTolerationSeconds", "MutatingAdmissionWebhook",
		"ValidatingAdmissionWebhook", "ResourceQuota",
	}
	if len(cfg.APIServer.AdmissionPlugins) == 0 { // If user provided an empty list, fill with defaults
		cfg.APIServer.AdmissionPlugins = append(cfg.APIServer.AdmissionPlugins, defaultAdmissionPlugins...)
	} else { // If user provided some, ensure defaults are there if not already present by user
		currentPluginsMap := make(map[string]bool)
		for _, plugin := range cfg.APIServer.AdmissionPlugins {
			currentPluginsMap[plugin] = true
		}
		for _, defaultPlugin := range defaultAdmissionPlugins {
			if !currentPluginsMap[defaultPlugin] {
				cfg.APIServer.AdmissionPlugins = append(cfg.APIServer.AdmissionPlugins, defaultPlugin)
			}
		}
	}

	if cfg.APIServer.ServiceNodePortRange == "" { cfg.APIServer.ServiceNodePortRange = "30000-32767" }

	if cfg.ControllerManager == nil { cfg.ControllerManager = &ControllerManagerConfig{} }
	if cfg.ControllerManager.ExtraArgs == nil { cfg.ControllerManager.ExtraArgs = []string{} }
	defaultControllerManagerArgs := map[string]string{
		"--profiling":                       "--profiling=false",
		"--terminated-pod-gc-threshold":   "--terminated-pod-gc-threshold=12500",
		"--leader-elect":                  "--leader-elect=true",
		"--use-service-account-credentials": "--use-service-account-credentials=true",
		"--bind-address":                  "--bind-address=127.0.0.1",
		// --service-account-private-key-file and --root-ca-file are runtime-dependent
	}
	cfg.ControllerManager.ExtraArgs = util.EnsureExtraArgs(cfg.ControllerManager.ExtraArgs, defaultControllerManagerArgs)

	if cfg.Scheduler == nil { cfg.Scheduler = &SchedulerConfig{} }
	if cfg.Scheduler.ExtraArgs == nil { cfg.Scheduler.ExtraArgs = []string{} }
	defaultSchedulerArgs := map[string]string{
		"--profiling":    "--profiling=false",
		"--leader-elect": "--leader-elect=true",
		"--bind-address": "--bind-address=127.0.0.1",
	}
	cfg.Scheduler.ExtraArgs = util.EnsureExtraArgs(cfg.Scheduler.ExtraArgs, defaultSchedulerArgs)

	if cfg.Kubelet == nil { cfg.Kubelet = &KubeletConfig{} }
	SetDefaults_KubeletConfig(cfg.Kubelet, cfg.ContainerManager) // This will handle Kubelet.ExtraArgs

	if cfg.KubeProxy == nil { cfg.KubeProxy = &KubeProxyConfig{} }
	if cfg.KubeProxy.ExtraArgs == nil { cfg.KubeProxy.ExtraArgs = []string{} }
	if cfg.ProxyMode == common.KubeProxyModeIPTables {
		if cfg.KubeProxy.IPTables == nil { cfg.KubeProxy.IPTables = &KubeProxyIPTablesConfig{} }
		SetDefaults_KubeProxyIPTablesConfig(cfg.KubeProxy.IPTables)
	}
	if cfg.ProxyMode == common.KubeProxyModeIPVS { // Use common constant
		if cfg.KubeProxy.IPVS == nil { cfg.KubeProxy.IPVS = &KubeProxyIPVSConfig{} }
		SetDefaults_KubeProxyIPVSConfig(cfg.KubeProxy.IPVS)
	}
}

func SetDefaults_KubeProxyIPTablesConfig(cfg *KubeProxyIPTablesConfig) {
	if cfg == nil { return }
	if cfg.MasqueradeAll == nil { cfg.MasqueradeAll = util.BoolPtr(false) } // Changed default to false
	if cfg.MasqueradeBit == nil { cfg.MasqueradeBit = util.Int32Ptr(14) }
	if cfg.SyncPeriod == "" { cfg.SyncPeriod = "30s" }
	if cfg.MinSyncPeriod == "" { cfg.MinSyncPeriod = "15s" }
}

func SetDefaults_KubeProxyIPVSConfig(cfg *KubeProxyIPVSConfig) {
	if cfg == nil { return }
	if cfg.Scheduler == "" { cfg.Scheduler = "rr" }
	if cfg.ExcludeCIDRs == nil { cfg.ExcludeCIDRs = []string{} }
	if cfg.SyncPeriod == "" { cfg.SyncPeriod = "30s" }
	if cfg.MinSyncPeriod == "" { cfg.MinSyncPeriod = "15s" }
}

func SetDefaults_KubeletConfig(cfg *KubeletConfig, containerManager string) {
	if cfg == nil { return }
	if cfg.ExtraArgs == nil { cfg.ExtraArgs = []string{} }
	defaultKubeletArgs := map[string]string{
		"--anonymous-auth":                  "--anonymous-auth=false",
		"--authorization-mode":              "--authorization-mode=Webhook",
		"--read-only-port":                  "--read-only-port=0",
		"--streaming-connection-idle-timeout": "--streaming-connection-idle-timeout=4h0m0s",
		"--protect-kernel-defaults":         "--protect-kernel-defaults=true",
		"--event-qps":                       "--event-qps=0",
		"--authentication-token-webhook":    "--authentication-token-webhook=true",
		"--feature-gates":                   "--feature-gates=RotateKubeletServerCertificate=true", // Note: this will merge if user also has feature-gates
		"--rotate-certificates":             "--rotate-certificates",
		// --client-ca-file, --kubeconfig are runtime-dependent
	}
	cfg.ExtraArgs = util.EnsureExtraArgs(cfg.ExtraArgs, defaultKubeletArgs)

	if cfg.EvictionHard == nil {
		cfg.EvictionHard = map[string]string{
			"memory.available":  "100Mi",
			"nodefs.available":  "10%",
			"imagefs.available": "15%",
			"nodefs.inodesFree": "5%",
		}
	}
	if cfg.PodPidsLimit == nil { cfg.PodPidsLimit = util.Int64Ptr(10000) }
	if cfg.CgroupDriver == nil {
		if containerManager != "" { cfg.CgroupDriver = util.StrPtr(containerManager)
		} else { cfg.CgroupDriver = util.StrPtr(common.CgroupDriverSystemd) } // Use common constant
	}
	if cfg.HairpinMode == nil {
		cfg.HairpinMode = util.StrPtr(common.DefaultKubeletHairpinMode)
	}
}

func Validate_KubernetesConfig(cfg *KubernetesConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { verrs.Add(pathPrefix, "kubernetes configuration section cannot be nil"); return }
	if !util.ContainsString(validK8sTypes, cfg.Type) {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("invalid type '%s', must be one of %v or empty for default", cfg.Type, validK8sTypes))
	}
	if strings.TrimSpace(cfg.Version) == "" {
		verrs.Add(pathPrefix+".version", "cannot be empty")
	} else if !util.IsValidRuntimeVersion(cfg.Version) {
		verrs.Add(pathPrefix+".version", fmt.Sprintf("'%s' is not a recognized version format", cfg.Version))
	}
	if strings.TrimSpace(cfg.DNSDomain) == "" { verrs.Add(pathPrefix+".dnsDomain", "cannot be empty") }
	if !util.ContainsString(validProxyModes, cfg.ProxyMode) {
		verrs.Add(pathPrefix+".proxyMode", fmt.Sprintf("invalid mode '%s', must be one of %v or empty for default", cfg.ProxyMode, validProxyModes))
	}
	if cfg.ContainerRuntime != nil { Validate_ContainerRuntimeConfig(cfg.ContainerRuntime, verrs, pathPrefix+".containerRuntime")
	} else { verrs.Add(pathPrefix+".containerRuntime", "section cannot be nil") }
	if cfg.APIServer != nil { Validate_APIServerConfig(cfg.APIServer, verrs, pathPrefix+".apiServer") }
	if cfg.ControllerManager != nil { Validate_ControllerManagerConfig(cfg.ControllerManager, verrs, pathPrefix+".controllerManager") }
	if cfg.Scheduler != nil { Validate_SchedulerConfig(cfg.Scheduler, verrs, pathPrefix+".scheduler") }
	if cfg.Kubelet != nil { Validate_KubeletConfig(cfg.Kubelet, verrs, pathPrefix+".kubelet") }
	if cfg.KubeProxy != nil { Validate_KubeProxyConfig(cfg.KubeProxy, verrs, pathPrefix+".kubeProxy", cfg.ProxyMode) }
	if cfg.ContainerManager != "" && !util.ContainsString(validKubeletCgroupDrivers, cfg.ContainerManager) {
		verrs.Add(pathPrefix+".containerManager", fmt.Sprintf("must be one of %v, got '%s'", validKubeletCgroupDrivers, cfg.ContainerManager))
	}
	if cfg.KubeletConfiguration != nil && len(cfg.KubeletConfiguration.Raw) == 0 {
		verrs.Add(pathPrefix+".kubeletConfiguration", "raw data cannot be empty if section is present")
	}
	if cfg.KubeProxyConfiguration != nil && len(cfg.KubeProxyConfiguration.Raw) == 0 {
		verrs.Add(pathPrefix+".kubeProxyConfiguration", "raw data cannot be empty if section is present")
	}

	// Validate ApiserverCertExtraSans
	for i, san := range cfg.ApiserverCertExtraSans {
		trimmedSan := strings.TrimSpace(san)
		if trimmedSan == "" {
			verrs.Add(fmt.Sprintf("%s.apiserverCertExtraSans[%d]", pathPrefix, i), "SAN entry cannot be empty")
		} else if !util.IsValidIP(trimmedSan) && !util.IsValidDomainName(trimmedSan) {
			verrs.Add(fmt.Sprintf("%s.apiserverCertExtraSans[%d]", pathPrefix, i), fmt.Sprintf("invalid SAN entry '%s', must be a valid IP address or DNS name", san))
		}
	}
}

func Validate_APIServerConfig(cfg *APIServerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	for i, san := range cfg.EtcdServers { // Assuming EtcdServers might contain FQDNs or IPs
		if strings.TrimSpace(san) == "" {
			verrs.Add(fmt.Sprintf("%s.etcdServers[%d]", pathPrefix, i), "etcd server entry cannot be empty")
		}
		// Add more specific validation if needed, e.g. URL format, but often these are just host:port or URLs
	}
	if cfg.EtcdCAFile != "" && strings.TrimSpace(cfg.EtcdCAFile) == "" {
		verrs.Add(pathPrefix+".etcdCAFile", "cannot be only whitespace if specified")
	}
	if cfg.EtcdCertFile != "" && strings.TrimSpace(cfg.EtcdCertFile) == "" {
		verrs.Add(pathPrefix+".etcdCertFile", "cannot be only whitespace if specified")
	}
	if cfg.EtcdKeyFile != "" && strings.TrimSpace(cfg.EtcdKeyFile) == "" {
		verrs.Add(pathPrefix+".etcdKeyFile", "cannot be only whitespace if specified")
	}

	if cfg.ServiceNodePortRange != "" {
		parts := strings.Split(cfg.ServiceNodePortRange, "-")
		if len(parts) != 2 {
			verrs.Add(pathPrefix+".serviceNodePortRange", fmt.Sprintf("invalid format '%s', expected 'min-max'", cfg.ServiceNodePortRange))
		} else {
			minPort, errMin := strconv.Atoi(parts[0])
			maxPort, errMax := strconv.Atoi(parts[1])
			if errMin != nil || errMax != nil {
				verrs.Add(pathPrefix+".serviceNodePortRange", fmt.Sprintf("ports must be numbers, got '%s'", cfg.ServiceNodePortRange))
			} else {
				if minPort <= 0 || minPort > 65535 || maxPort <= 0 || maxPort > 65535 {
					verrs.Add(pathPrefix+".serviceNodePortRange", fmt.Sprintf("port numbers must be between 1 and 65535, got min %d, max %d", minPort, maxPort))
				}
				if minPort >= maxPort {
					verrs.Add(pathPrefix+".serviceNodePortRange", fmt.Sprintf("min port %d must be less than max port %d", minPort, maxPort))
				}
			}
		}
	}
	if cfg.AdmissionPlugins != nil {
		for i, plugin := range cfg.AdmissionPlugins {
			if strings.TrimSpace(plugin) == "" {
				verrs.Add(fmt.Sprintf("%s.admissionPlugins[%d]", pathPrefix, i), "admission plugin name cannot be empty")
			}
		}
	}
	// Validate ApiserverCertExtraSans
	// This field is part of KubernetesConfig, not APIServerConfig directly in the struct,
	// but it's validated conceptually with APIServer settings. Let's adjust Validate_KubernetesConfig for it.
}

func Validate_ControllerManagerConfig(cfg *ControllerManagerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.ServiceAccountPrivateKeyFile != "" && strings.TrimSpace(cfg.ServiceAccountPrivateKeyFile) == "" {
		verrs.Add(pathPrefix+".serviceAccountPrivateKeyFile", "cannot be empty if specified")
	}
}
func Validate_SchedulerConfig(cfg *SchedulerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.PolicyConfigFile != "" && strings.TrimSpace(cfg.PolicyConfigFile) == "" {
		verrs.Add(pathPrefix+".policyConfigFile", "cannot be empty if specified")
	}
}

func Validate_KubeletConfig(cfg *KubeletConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.CgroupDriver != nil && !util.ContainsString(validKubeletCgroupDrivers, *cfg.CgroupDriver) {
	   verrs.Add(pathPrefix+".cgroupDriver", fmt.Sprintf("must be one of %v if specified, got '%s'", validKubeletCgroupDrivers, *cfg.CgroupDriver))
	}
	if cfg.HairpinMode != nil && *cfg.HairpinMode != "" && !util.ContainsString(validKubeletHairpinModes, *cfg.HairpinMode) {
		verrs.Add(pathPrefix+".hairpinMode", fmt.Sprintf("invalid mode '%s', must be one of %v or empty for default", *cfg.HairpinMode, validKubeletHairpinModes))
	}
	if cfg.PodPidsLimit != nil && *cfg.PodPidsLimit <= 0 && *cfg.PodPidsLimit != -1 {
		verrs.Add(pathPrefix+".podPidsLimit", fmt.Sprintf("must be positive or -1 (unlimited), got %d", *cfg.PodPidsLimit))
	}
}

func Validate_KubeProxyIPTablesConfig(cfg *KubeProxyIPTablesConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.MasqueradeBit != nil && (*cfg.MasqueradeBit < 0 || *cfg.MasqueradeBit > 31) {
		verrs.Add(pathPrefix+".masqueradeBit", fmt.Sprintf("must be between 0 and 31, got %d", *cfg.MasqueradeBit))
	}
	if cfg.SyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.SyncPeriod); err != nil {
			verrs.Add(pathPrefix+".syncPeriod", fmt.Sprintf("invalid duration format '%s': %v", cfg.SyncPeriod, err))
		}
	}
	if cfg.MinSyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.MinSyncPeriod); err != nil {
			verrs.Add(pathPrefix+".minSyncPeriod", fmt.Sprintf("invalid duration format '%s': %v", cfg.MinSyncPeriod, err))
		}
	}
}

func Validate_KubeProxyIPVSConfig(cfg *KubeProxyIPVSConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	for i, cidr := range cfg.ExcludeCIDRs {
		if !util.IsValidCIDR(cidr) {
			verrs.Add(fmt.Sprintf("%s.excludeCIDRs[%d]", pathPrefix, i), fmt.Sprintf("invalid CIDR format '%s'", cidr))
		}
	}
	if cfg.SyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.SyncPeriod); err != nil {
			verrs.Add(pathPrefix+".syncPeriod", fmt.Sprintf("invalid duration format '%s': %v", cfg.SyncPeriod, err))
		}
	}
	if cfg.MinSyncPeriod != "" {
		if _, err := time.ParseDuration(cfg.MinSyncPeriod); err != nil {
			verrs.Add(pathPrefix+".minSyncPeriod", fmt.Sprintf("invalid duration format '%s': %v", cfg.MinSyncPeriod, err))
		}
	}
}

func Validate_KubeProxyConfig(cfg *KubeProxyConfig, verrs *validation.ValidationErrors, pathPrefix string, parentProxyMode string) {
	if cfg == nil { return }

	if parentProxyMode == common.KubeProxyModeIPTables { // Use common constant
		if cfg.IPTables != nil {
			Validate_KubeProxyIPTablesConfig(cfg.IPTables, verrs, pathPrefix+".ipTables")
		}
		if cfg.IPVS != nil {
			verrs.Add(pathPrefix+".ipvs", "should not be set if proxyMode is 'iptables'")
		}
	} else if parentProxyMode == common.KubeProxyModeIPVS { // Use common constant
		if cfg.IPVS != nil {
			Validate_KubeProxyIPVSConfig(cfg.IPVS, verrs, pathPrefix+".ipvs")
		}
		if cfg.IPTables != nil {
			verrs.Add(pathPrefix+".ipTables", "should not be set if proxyMode is 'ipvs'")
		}
	}
}

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
