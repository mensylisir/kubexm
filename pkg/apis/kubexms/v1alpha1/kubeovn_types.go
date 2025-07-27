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

type KubeOvnConfig struct {
	Source           AddonSource              `json:"source,omitempty" yaml:"sources,omitempty"`
	Networking       *KubeOvnNetworking       `json:"networking,omitempty" yaml:"networking,omitempty"`
	Controller       *KubeOvnControllerConfig `json:"controller,omitempty" yaml:"controller,omitempty"`
	AdvancedFeatures *KubeOvnAdvancedFeatures `json:"advancedFeatures,omitempty" yaml:"advancedFeatures,omitempty"`
}

type KubeOvnNetworking struct {
	TunnelType string `json:"tunnelType,omitempty" yaml:"tunnelType,omitempty"`
	PodGateway string `json:"podGateway,omitempty" yaml:"podGateway,omitempty"`
	MTU        *int   `json:"mtu,omitempty" yaml:"mtu,omitempty"`
}

type KubeOvnControllerConfig struct {
	JoinCIDR             string `json:"joinCIDR,omitempty" yaml:"joinCIDR,omitempty"`
	NodeSwitchCIDR       string `json:"nodeSwitchCIDR,omitempty" yaml:"nodeSwitchCIDR,omitempty"`
	PodDefaultSubnetCIDR string `json:"podDefaultSubnetCIDR,omitempty" yaml:"podDefaultSubnetCIDR,omitempty"`
}

type KubeOvnAdvancedFeatures struct {
	EnableSSL           *bool `json:"enableSSL,omitempty" yaml:"enableSSL,omitempty"`
	EnableVPCNATGateway *bool `json:"enableVPCNATGateway,omitempty" yaml:"enableVPCNATGateway,omitempty"`
	EnableSubnetQoS     *bool `json:"enableSubnetQoS,omitempty" yaml:"enableSubnetQoS,omitempty"`
}

func SetDefaults_KubeOvnConfig(cfg *KubeOvnConfig) {
	if cfg == nil {
		return
	}
	if cfg.Networking == nil {
		cfg.Networking = &KubeOvnNetworking{}
	}
	if cfg.Networking.TunnelType == "" {
		cfg.Networking.TunnelType = common.KubeOvnNetworkingTunnelTypeGeneve
	}
	if cfg.Networking.MTU == nil {
		cfg.Networking.MTU = helpers.IntPtr(common.DefaultKubeOvnNetworkingMTU)
	}

	if cfg.Controller == nil {
		cfg.Controller = &KubeOvnControllerConfig{}
	}

	if cfg.AdvancedFeatures == nil {
		cfg.AdvancedFeatures = &KubeOvnAdvancedFeatures{}
	}
	if cfg.AdvancedFeatures.EnableSSL == nil {
		cfg.AdvancedFeatures.EnableSSL = helpers.BoolPtr(common.KubeOvnAdvancedFeaturesEnableSSL)
	}
	if cfg.AdvancedFeatures.EnableVPCNATGateway == nil {
		cfg.AdvancedFeatures.EnableVPCNATGateway = helpers.BoolPtr(common.KubeOvnAdvancedFeaturesEnableVPCNATGateway)
	}
	if cfg.AdvancedFeatures.EnableSubnetQoS == nil {
		cfg.AdvancedFeatures.EnableSubnetQoS = helpers.BoolPtr(common.KubeOvnAdvancedFeaturesEnableSubnetQoS)
	}
}

func Validate_KubeOvnConfig(cfg *KubeOvnConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Networking != nil {
		p := path.Join(pathPrefix, "networking")
		if !helpers.ContainsStringWithEmpty(common.ValidKubeOvnTunnelTypes, cfg.Networking.TunnelType) {
			verrs.Add(fmt.Sprintf("%s.tunnelType: invalid type '%s', must be one of [%s] or empty",
				p, cfg.Networking.TunnelType, strings.Join(common.ValidKubeOvnTunnelTypes, ", ")))
		}
		if cfg.Networking.PodGateway != "" {
			if net.ParseIP(cfg.Networking.PodGateway) == nil {
				verrs.Add(fmt.Sprintf("%s.podGateway: invalid IP address format for '%s'", p, cfg.Networking.PodGateway))
			}
		}
	}

	if cfg.Controller != nil {
		p := path.Join(pathPrefix, "controller")
		if cfg.Controller.JoinCIDR != "" {
			if _, _, err := net.ParseCIDR(cfg.Controller.JoinCIDR); err != nil {
				verrs.Add(fmt.Sprintf("%s.joinCIDR: invalid CIDR format for '%s': %v", p, cfg.Controller.JoinCIDR, err))
			}
		} else {
			verrs.Add(fmt.Sprintf("%s.joinCIDR: is a required field", p))
		}

		if cfg.Controller.NodeSwitchCIDR != "" {
			if _, _, err := net.ParseCIDR(cfg.Controller.NodeSwitchCIDR); err != nil {
				verrs.Add(fmt.Sprintf("%s.nodeSwitchCIDR: invalid CIDR format for '%s': %v", p, cfg.Controller.NodeSwitchCIDR, err))
			}
		} else {
			verrs.Add(fmt.Sprintf("%s.nodeSwitchCIDR: is a required field", p))
		}

		if cfg.Controller.PodDefaultSubnetCIDR != "" {
			if _, _, err := net.ParseCIDR(cfg.Controller.PodDefaultSubnetCIDR); err != nil {
				verrs.Add(fmt.Sprintf("%s.podDefaultSubnetCIDR: invalid CIDR format for '%s': %v", p, cfg.Controller.PodDefaultSubnetCIDR, err))
			}
		} else {
			verrs.Add(fmt.Sprintf("%s.podDefaultSubnetCIDR: is a required field", p))
		}
	} else {
		verrs.Add(fmt.Sprintf("%s.controller: is a required section", pathPrefix))
	}
}
