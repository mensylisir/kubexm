package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"net"
	"path"
	"strings"
)

type HybridnetConfig struct {
	Installation   *HybridnetInstallationConfig   `json:"installation,omitempty" yaml:"installation,omitempty"`
	DefaultNetwork *HybridnetDefaultNetworkConfig `json:"defaultNetwork,omitempty"yaml:"defaultNetwork,omitempty"`
	Features       *HybridnetFeaturesConfig       `json:"features,omitempty" yaml:"features,omitempty"`
}

type HybridnetInstallationConfig struct {
	Enabled       *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ImageRegistry string `json:"imageRegistry,omitempty" yaml:"imageRegistry,omitempty"`
}

type HybridnetDefaultNetworkConfig struct {
	CreateOnInit              *bool  `json:"createOnInit,omitempty" yaml:"createOnInit,omitempty"`
	Type                      string `json:"type,omitempty" yaml:"type,omitempty"`
	DefaultUnderlaySubnetName string `json:"defaultUnderlaySubnetName,omitempty" yaml:"defaultUnderlaySubnetName,omitempty"`
	DefaultOverlaySubnetCIDR  string `json:"defaultOverlaySubnetCIDR,omitempty" yaml:"defaultOverlaySubnetCIDR,omitempty"`
}

type HybridnetFeaturesConfig struct {
	EnableNetworkPolicy *bool `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	EnableMultiCluster  *bool `json:"enableMultiCluster,omitempty" yaml:"enableMultiCluster,omitempty"`
}

func SetDefaults_HybridnetConfig(cfg *HybridnetConfig) {
	if cfg == nil {
		return
	}

	if cfg.Installation == nil {
		cfg.Installation = &HybridnetInstallationConfig{}
	}
	if cfg.Installation.Enabled == nil {
		cfg.Installation.Enabled = helpers.BoolPtr(common.HybridnetInstallationConfigEnabled)
	}

	if cfg.DefaultNetwork == nil {
		cfg.DefaultNetwork = &HybridnetDefaultNetworkConfig{}
	}
	if cfg.DefaultNetwork.CreateOnInit == nil {
		cfg.DefaultNetwork.CreateOnInit = helpers.BoolPtr(common.HybridnetDefaultNetworkConfigCreateOnInit)
	}
	if cfg.DefaultNetwork.Type == "" {
		cfg.DefaultNetwork.Type = common.DefaultHybridnetDefaultNetworkConfigType
	}

	if cfg.Features == nil {
		cfg.Features = &HybridnetFeaturesConfig{}
	}
	if cfg.Features.EnableNetworkPolicy == nil {
		cfg.Features.EnableNetworkPolicy = helpers.BoolPtr(common.HybridnetFeaturesConfigEnableNetworkPolicy)
	}
	if cfg.Features.EnableMultiCluster == nil {
		cfg.Features.EnableMultiCluster = helpers.BoolPtr(common.HybridnetFeaturesConfigEnableMultiCluster)
	}
}

func Validate_HybridnetConfig(cfg *HybridnetConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Installation != nil && cfg.Installation.Enabled != nil && !*cfg.Installation.Enabled {
		return
	}

	if cfg.DefaultNetwork != nil {
		p := path.Join(pathPrefix, "defaultNetwork")
		if !helpers.ContainsStringWithEmpty(common.ValidHybridnetNetworkTypes, cfg.DefaultNetwork.Type) {
			verrs.Add(fmt.Sprintf("%s.type: invalid type '%s', must be one of [%s] or empty",
				p, cfg.DefaultNetwork.Type, strings.Join(common.ValidHybridnetNetworkTypes, ", ")))
		}

		networkType := cfg.DefaultNetwork.Type
		if networkType == "Overlay" {
			if cfg.DefaultNetwork.DefaultOverlaySubnetCIDR == "" {
				verrs.Add(fmt.Sprintf("%s.defaultOverlaySubnetCIDR: is required when type is 'Overlay'", p))
			} else {
				if _, _, err := net.ParseCIDR(cfg.DefaultNetwork.DefaultOverlaySubnetCIDR); err != nil {
					verrs.Add(fmt.Sprintf("%s.defaultOverlaySubnetCIDR: invalid CIDR format '%s': %v", p, cfg.DefaultNetwork.DefaultOverlaySubnetCIDR, err))
				}
			}
		}

		if networkType == "Underlay" {
			if cfg.DefaultNetwork.DefaultUnderlaySubnetName == "" {
				verrs.Add(fmt.Sprintf("%s.defaultUnderlaySubnetName: is required when type is 'Underlay'", p))
			}
		}
	} else {
		verrs.Add(fmt.Sprintf("%s.defaultNetwork: configuration is required when hybridnet is enabled", pathPrefix))
	}
}
