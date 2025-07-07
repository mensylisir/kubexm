package v1alpha1

import (
	"fmt"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/mensylisir/kubexm/pkg/common" // Import common
	"github.com/mensylisir/kubexm/pkg/util"   // Import util
)

// ExternalLoadBalancerConfig defines settings for an external load balancing solution.
type ExternalLoadBalancerConfig struct {
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	Keepalived *KeepalivedConfig `json:"keepalived,omitempty" yaml:"keepalived,omitempty"`
	HAProxy    *HAProxyConfig    `json:"haproxy,omitempty" yaml:"haproxy,omitempty"`
	NginxLB    *NginxLBConfig    `json:"nginxLB,omitempty" yaml:"nginxLB,omitempty"`
	LoadBalancerHostGroupName *string `json:"loadBalancerHostGroupName,omitempty" yaml:"loadBalancerHostGroupName,omitempty"`
}

// InternalLoadBalancerConfig defines settings for an internal load balancing solution.
type InternalLoadBalancerConfig struct {
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	KubeVIP           *KubeVIPConfig `json:"kubevip,omitempty" yaml:"kubevip,omitempty"`
	WorkerNodeHAProxy *HAProxyConfig `json:"workerNodeHAProxy,omitempty" yaml:"workerNodeHAProxy,omitempty"`
}

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	Enabled  *bool                       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	External *ExternalLoadBalancerConfig `json:"external,omitempty" yaml:"external,omitempty"`
	Internal *InternalLoadBalancerConfig `json:"internal,omitempty" yaml:"internal,omitempty"`
}

func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = util.BoolPtr(false)
	}

	if !*cfg.Enabled {
		cfg.External = nil
		cfg.Internal = nil
		return
	}

	if cfg.External != nil {
		SetDefaults_ExternalLoadBalancerConfig(cfg.External)
	}
	if cfg.Internal != nil {
		SetDefaults_InternalLoadBalancerConfig(cfg.Internal)
	}
}

func SetDefaults_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type != "" {
		isKeepalivedType := strings.Contains(cfg.Type, "Keepalived") || cfg.Type == common.ExternalLBTypeKubexmKH || cfg.Type == common.ExternalLBTypeKubexmKN
		isHAProxyType := strings.Contains(cfg.Type, "HAProxy") || cfg.Type == common.ExternalLBTypeKubexmKH
		isNginxLBType := strings.Contains(cfg.Type, "NginxLB") || cfg.Type == common.ExternalLBTypeKubexmKN

		if isKeepalivedType {
			if cfg.Keepalived == nil {
				cfg.Keepalived = &KeepalivedConfig{}
			}
			SetDefaults_KeepalivedConfig(cfg.Keepalived)
		}
		if isHAProxyType {
			if cfg.HAProxy == nil {
				cfg.HAProxy = &HAProxyConfig{}
			}
			SetDefaults_HAProxyConfig(cfg.HAProxy)
		}
		if isNginxLBType {
			if cfg.NginxLB == nil {
				cfg.NginxLB = &NginxLBConfig{}
			}
			SetDefaults_NginxLBConfig(cfg.NginxLB)
		}
	}
}

func SetDefaults_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == common.InternalLBTypeKubeVIP {
		if cfg.KubeVIP == nil {
			cfg.KubeVIP = &KubeVIPConfig{}
		}
		SetDefaults_KubeVIPConfig(cfg.KubeVIP)
	}
	// Assuming "WorkerNodeHAProxy" should map to common.InternalLBTypeHAProxy
	if cfg.Type == common.InternalLBTypeHAProxy || cfg.Type == "WorkerNodeHAProxy" {
		if cfg.WorkerNodeHAProxy == nil {
			cfg.WorkerNodeHAProxy = &HAProxyConfig{}
		}
		SetDefaults_HAProxyConfig(cfg.WorkerNodeHAProxy)
	}
}

func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Enabled != nil && *cfg.Enabled {
		isExternalConfigured := cfg.External != nil && cfg.External.Type != ""
		isInternalConfigured := cfg.Internal != nil && cfg.Internal.Type != ""

		if isExternalConfigured && isInternalConfigured {
			verrs.Add(pathPrefix, "external load balancer and internal load balancer cannot be enabled simultaneously")
		}

		if cfg.External != nil {
			Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external")
		}
		if cfg.Internal != nil {
			Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
		}
	} else {
		if cfg.External != nil && cfg.External.Type != "" {
			verrs.Add(pathPrefix+".external", "cannot be configured if global HA is disabled")
		}
		if cfg.Internal != nil && cfg.Internal.Type != "" {
			verrs.Add(pathPrefix+".internal", "cannot be configured if global HA is disabled")
		}
	}
}

func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Type == "" {
		return
	}

	// Define known external LB types and their properties
	isLegacyManagedKeepalivedHAProxy := cfg.Type == "ManagedKeepalivedHAProxy"
	isLegacyManagedKeepalivedNginxLB := cfg.Type == "ManagedKeepalivedNginxLB"
	isKubexmManagedKH := cfg.Type == common.ExternalLBTypeKubexmKH
	isKubexmManagedKN := cfg.Type == common.ExternalLBTypeKubexmKN
	isUserProvided := cfg.Type == "UserProvided"
	isGenericExternal := cfg.Type == common.ExternalLBTypeExternal

	isManagedType := isLegacyManagedKeepalivedHAProxy || isLegacyManagedKeepalivedNginxLB || isKubexmManagedKH || isKubexmManagedKN
	isValidKnownType := isManagedType || isUserProvided || isGenericExternal

	if !isValidKnownType {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("unknown external LB type '%s'", cfg.Type))
		return
	}

	if isUserProvided || isGenericExternal {
		if cfg.Keepalived != nil {
			verrs.Add(pathPrefix+".keepalived", fmt.Sprintf("should not be set for '%s' external LB type", cfg.Type))
		}
		if cfg.HAProxy != nil {
			verrs.Add(pathPrefix+".haproxy", fmt.Sprintf("should not be set for '%s' external LB type", cfg.Type))
		}
		if cfg.NginxLB != nil {
			verrs.Add(pathPrefix+".nginxLB", fmt.Sprintf("should not be set for '%s' external LB type", cfg.Type))
		}
	}

	if isManagedType {
		if cfg.Keepalived == nil {
			verrs.Add(pathPrefix+".keepalived", "section must be present for managed LB type '"+cfg.Type+"'")
		} else {
			Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived")
		}

		if isLegacyManagedKeepalivedHAProxy || isKubexmManagedKH {
			if cfg.HAProxy == nil {
				verrs.Add(pathPrefix+".haproxy", "section must be present for HAProxy based managed LB type '"+cfg.Type+"'")
			} else {
				Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy")
			}
		} else if isLegacyManagedKeepalivedNginxLB || isKubexmManagedKN {
			if cfg.NginxLB == nil {
				verrs.Add(pathPrefix+".nginxLB", "section must be present for NginxLB based managed LB type '"+cfg.Type+"'")
			} else {
				Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB")
			}
		}

		if cfg.LoadBalancerHostGroupName == nil || strings.TrimSpace(*cfg.LoadBalancerHostGroupName) == "" {
			verrs.Add(pathPrefix+".loadBalancerHostGroupName", "must be specified for managed external LB type '"+cfg.Type+"'")
		}
	}
}

func Validate_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Type == "" {
		return
	}

	isKubeVIP := cfg.Type == common.InternalLBTypeKubeVIP || cfg.Type == "KubeVIP" // Allow legacy string "KubeVIP"
	isHAProxy := cfg.Type == common.InternalLBTypeHAProxy || cfg.Type == "WorkerNodeHAProxy" // Allow legacy string "WorkerNodeHAProxy"

	if isKubeVIP {
		if cfg.KubeVIP == nil {
			// This case should ideally be caught by SetDefaults ensuring KubeVIP is initialized if type is KubeVIP.
			// However, direct validation without prior defaulting should also be robust.
			verrs.Add(pathPrefix+".kubevip", "section must be present if type is '"+cfg.Type+"'")
			// No further validation on cfg.KubeVIP if it's nil.
		} else {
			Validate_KubeVIPConfig(cfg.KubeVIP, verrs, pathPrefix+".kubevip")
		}
	} else if isHAProxy {
		if cfg.WorkerNodeHAProxy == nil {
			verrs.Add(pathPrefix+".workerNodeHAProxy", "section must be present if type is '"+cfg.Type+"'")
		} else {
			Validate_HAProxyConfig(cfg.WorkerNodeHAProxy, verrs, pathPrefix+".workerNodeHAProxy")
		}
	} else if cfg.Type != "" { // Only error if type is non-empty and not recognized
		verrs.Add(pathPrefix+".type", fmt.Sprintf("unknown internal LB type '%s'", cfg.Type))
	}
	// If cfg.Type is empty, it's considered valid (no internal LB explicitly chosen).
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalLoadBalancerConfig) DeepCopyInto(out *ExternalLoadBalancerConfig) {
	*out = *in
	if in.Keepalived != nil {
		out.Keepalived = new(KeepalivedConfig)
		in.Keepalived.DeepCopyInto(out.Keepalived)
	}
	if in.HAProxy != nil {
		out.HAProxy = new(HAProxyConfig)
		in.HAProxy.DeepCopyInto(out.HAProxy)
	}
	if in.NginxLB != nil {
		out.NginxLB = new(NginxLBConfig)
		in.NginxLB.DeepCopyInto(out.NginxLB)
	}
	if in.LoadBalancerHostGroupName != nil {
		in, out := &in.LoadBalancerHostGroupName, &out.LoadBalancerHostGroupName
		*out = new(string)
		**out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalLoadBalancerConfig.
func (in *ExternalLoadBalancerConfig) DeepCopy() *ExternalLoadBalancerConfig {
	if in == nil {
		return nil
	}
	out := new(ExternalLoadBalancerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InternalLoadBalancerConfig) DeepCopyInto(out *InternalLoadBalancerConfig) {
	*out = *in
	if in.KubeVIP != nil {
		out.KubeVIP = new(KubeVIPConfig)
		in.KubeVIP.DeepCopyInto(out.KubeVIP)
	}
	if in.WorkerNodeHAProxy != nil {
		out.WorkerNodeHAProxy = new(HAProxyConfig)
		in.WorkerNodeHAProxy.DeepCopyInto(out.WorkerNodeHAProxy)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InternalLoadBalancerConfig.
func (in *InternalLoadBalancerConfig) DeepCopy() *InternalLoadBalancerConfig {
	if in == nil {
		return nil
	}
	out := new(InternalLoadBalancerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HighAvailabilityConfig) DeepCopyInto(out *HighAvailabilityConfig) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = *in
	}
	if in.External != nil {
		out.External = new(ExternalLoadBalancerConfig)
		in.External.DeepCopyInto(out.External)
	}
	if in.Internal != nil {
		out.Internal = new(InternalLoadBalancerConfig)
		in.Internal.DeepCopyInto(out.Internal)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HighAvailabilityConfig.
func (in *HighAvailabilityConfig) DeepCopy() *HighAvailabilityConfig {
	if in == nil {
		return nil
	}
	out := new(HighAvailabilityConfig)
	in.DeepCopyInto(out)
	return out
}
