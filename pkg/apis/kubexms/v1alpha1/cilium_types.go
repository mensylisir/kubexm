package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type CiliumConfig struct {
	Source      AddonSource              `json:"source,omitempty" yaml:"sources,omitempty"`
	Network     *CiliumNetworkConfig     `json:"network,omitempty" yaml:"network,omitempty"`
	KubeProxy   *CiliumKubeProxyConfig   `json:"kubeProxy,omitempty" yaml:"kubeProxy,omitempty"`
	Hubble      *CiliumHubbleConfig      `json:"hubble,omitempty" yaml:"hubble,omitempty"`
	Security    *CiliumSecurityConfig    `json:"security,omitempty" yaml:"security,omitempty"`
	Performance *CiliumPerformanceConfig `json:"performance,omitempty" yaml:"performance,omitempty"`
}

type CiliumNetworkConfig struct {
	TunnelingMode         string `json:"tunnelingMode,omitempty" yaml:"tunnelingMode,omitempty"`
	IPAMMode              string `json:"ipamMode,omitempty" yaml:"ipamMode,omitempty"`
	EnableBGPControlPlane *bool  `json:"enableBGPControlPlane,omitempty" yaml:"enableBGPControlPlane,omitempty"`
}

type CiliumKubeProxyConfig struct {
	ReplacementMode     string `json:"replacement,omitempty" yaml:"replacement,omitempty"`
	EnableBPFMasquerade *bool  `json:"enableBPFMasquerade,omitempty" yaml:"enableBPFMasquerade,omitempty"`
}

type CiliumHubbleConfig struct {
	Enable   bool `json:"enable,omitempty" yaml:"enable,omitempty"`
	EnableUI bool `json:"enableUI,omitempty" yaml:"enableUI,omitempty"`
}

type CiliumSecurityConfig struct {
	IdentityAllocationMode string `json:"identityAllocationMode,omitempty" yaml:"identityAllocationMode,omitempty"`
	EnableEncryption       *bool  `json:"enableEncryption,omitempty" yaml:"enableEncryption,omitempty"`
}

type CiliumPerformanceConfig struct {
	EnableBandwidthManager *bool `json:"enableBandwidthManager,omitempty" yaml:"enableBandwidthManager,omitempty"`
}

func SetDefaults_CiliumConfig(cfg *CiliumConfig) {
	if cfg == nil {
		return
	}
	if cfg.Network == nil {
		cfg.Network = &CiliumNetworkConfig{}
	}
	if cfg.Network.TunnelingMode == "" {
		cfg.Network.TunnelingMode = common.DefaultTunnelingMode
	}
	if cfg.Network.IPAMMode == "" {
		cfg.Network.IPAMMode = common.DefaultCiliumIPAMsMode
	}
	if cfg.Network.EnableBGPControlPlane == nil {
		cfg.Network.EnableBGPControlPlane = helpers.BoolPtr(common.DefaultCiliumBGPControlPlaneEnable)
	}

	if cfg.KubeProxy == nil {
		cfg.KubeProxy = &CiliumKubeProxyConfig{}
	}
	if cfg.KubeProxy.ReplacementMode == "" {
		cfg.KubeProxy.ReplacementMode = common.DefaultCiliumKPRModes
	}
	if cfg.KubeProxy.EnableBPFMasquerade == nil {
		cfg.KubeProxy.EnableBPFMasquerade = helpers.BoolPtr(common.DefaultEnableBPFMasqueradeEnable)
	}

	if cfg.Hubble == nil {
		cfg.Hubble = &CiliumHubbleConfig{}
	}
	if cfg.Hubble.EnableUI && !cfg.Hubble.Enable {
		cfg.Hubble.Enable = common.DefaultCiliumHubbleConfigEnable
	}

	if cfg.Security == nil {
		cfg.Security = &CiliumSecurityConfig{}
	}
	if cfg.Security.IdentityAllocationMode == "" {
		cfg.Security.IdentityAllocationMode = common.DefaultIdentityAllocationMode
	}
	if cfg.Security.EnableEncryption == nil {
		cfg.Security.EnableEncryption = helpers.BoolPtr(common.DefaultEnableEncryption)
	}

	if cfg.Performance == nil {
		cfg.Performance = &CiliumPerformanceConfig{}
	}
	if cfg.Performance.EnableBandwidthManager == nil {
		cfg.Performance.EnableBandwidthManager = helpers.BoolPtr(common.DefaultEnableBandwidthManager)
	}
}

func Validate_CiliumConfig(cfg *CiliumConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Network != nil {
		p := path.Join(pathPrefix, "network")
		if !helpers.ContainsStringWithEmpty(common.ValidCiliumTunnelModes, cfg.Network.TunnelingMode) {
			verrs.Add(fmt.Sprintf("%s.tunnelingMode: invalid mode '%s', must be one of [%s] or empty",
				p, cfg.Network.TunnelingMode, strings.Join(common.ValidCiliumTunnelModes, ", ")))
		}
		if !helpers.ContainsStringWithEmpty(common.ValidCiliumIPAMModes, cfg.Network.IPAMMode) {
			verrs.Add(fmt.Sprintf("%s.ipamMode: invalid mode '%s', must be one of [%s] or empty",
				p, cfg.Network.IPAMMode, strings.Join(common.ValidCiliumIPAMModes, ", ")))
		}
	}

	if cfg.KubeProxy != nil {
		p := path.Join(pathPrefix, "kubeProxy")
		if !helpers.ContainsStringWithEmpty(common.ValidCiliumKPRModes, cfg.KubeProxy.ReplacementMode) {
			verrs.Add(fmt.Sprintf("%s.replacement: invalid mode '%s', must be one of [%s] or empty",
				p, cfg.KubeProxy.ReplacementMode, strings.Join(common.ValidCiliumKPRModes, ", ")))
		}
	}

	if cfg.Hubble != nil {
		if cfg.Hubble.EnableUI && !cfg.Hubble.Enable {
			verrs.Add(fmt.Sprintf("%s.hubble.enableUI: cannot be true if hubble.enable is false", pathPrefix))
		}
	}

	if cfg.Security != nil {
		p := path.Join(pathPrefix, "security")
		if !helpers.ContainsStringWithEmpty(common.ValidCiliumIdentModes, cfg.Security.IdentityAllocationMode) {
			verrs.Add(fmt.Sprintf("%s.identityAllocationMode: invalid mode '%s', must be one of [%s] or empty",
				p, cfg.Security.IdentityAllocationMode, strings.Join(common.ValidCiliumIdentModes, ", ")))
		}
	}
}
