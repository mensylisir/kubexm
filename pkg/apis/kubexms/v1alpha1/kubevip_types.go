package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
)

type KubeVIPBGPConfig struct {
	RouterID      string `json:"routerID,omitempty" yaml:"routerID,omitempty"`
	PeerAddress   string `json:"peerAddress,omitempty" yaml:"peerAddress,omitempty"`
	PeerASN       uint32 `json:"peerASN,omitempty" yaml:"peerASN,omitempty"`
	ASN           uint32 `json:"asn,omitempty" yaml:"asn,omitempty"`
	SourceAddress string `json:"sourceAddress,omitempty" yaml:"sourceAddress,omitempty"`
}

type KubeVIPConfig struct {
	Mode                 *string           `json:"mode,omitempty" yaml:"mode,omitempty"`
	VIP                  *string           `json:"vip,omitempty" yaml:"vip,omitempty"`
	Interface            *string           `json:"interface,omitempty" yaml:"interface,omitempty"`
	EnableControlPlaneLB *bool             `json:"enableControlPlaneLB,omitempty" yaml:"enableControlPlaneLB,omitempty"`
	EnableServicesLB     *bool             `json:"enableServicesLB,omitempty" yaml:"enableServicesLB,omitempty"`
	Image                *string           `json:"image,omitempty" yaml:"image,omitempty"`
	ExtraArgs            []string          `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	BGPConfig            *KubeVIPBGPConfig `json:"bgpConfig,omitempty" yaml:"bgpConfig,omitempty"`
}

func SetDefaults_KubeVIPConfig(cfg *KubeVIPConfig) {
	if cfg == nil {
		return
	}
	if cfg.Mode == nil {
		cfg.Mode = helpers.StrPtr(common.DefaultKubeVIPMode)
	}
	if cfg.EnableControlPlaneLB == nil {
		cfg.EnableControlPlaneLB = helpers.BoolPtr(true)
	}
	if cfg.EnableServicesLB == nil {
		cfg.EnableServicesLB = helpers.BoolPtr(false)
	}
	if cfg.Image == nil || *cfg.Image == "" {
		cfg.Image = helpers.StrPtr(common.DefaultKubeVIPImage)
	}
	if cfg.ExtraArgs == nil {
		cfg.ExtraArgs = []string{}
	}
	if cfg.Mode != nil && *cfg.Mode == common.KubeVIPModeBGP && cfg.BGPConfig == nil {
		cfg.BGPConfig = &KubeVIPBGPConfig{}
	}
}

func Validate_KubeVIPConfig(cfg *KubeVIPConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Mode == nil {
		verrs.Add(pathPrefix+".mode", "is a required field")
		return
	}

	if !helpers.ContainsString(common.ValidKubeVIPModes, *cfg.Mode) {
		verrs.Add(pathPrefix+".mode", fmt.Sprintf("invalid mode '%s', must be one of %v", *cfg.Mode, common.ValidKubeVIPModes))
	}

	if !helpers.IsValidNonEmptyString(*cfg.VIP) {
		verrs.Add(pathPrefix+".vip", "virtual IP address must be specified")
	} else if !helpers.IsValidIP(*cfg.VIP) {
		verrs.Add(pathPrefix+".vip", fmt.Sprintf("invalid IP address format '%s'", *cfg.VIP))
	}

	if cfg.Image != nil && !helpers.IsValidNonEmptyString(*cfg.Image) {
		verrs.Add(pathPrefix+".image", "cannot be empty if specified")
	}

	switch *cfg.Mode {
	case common.KubeVIPModeARP:
		if !helpers.IsValidNonEmptyString(*cfg.Interface) {
			verrs.Add(pathPrefix+".interface", "network interface must be specified for ARP mode")
		}
	case common.KubeVIPModeBGP:
		if cfg.BGPConfig == nil {
			verrs.Add(pathPrefix+".bgpConfig", "BGP configuration must be provided for BGP mode")
		} else {
			Validate_KubeVIPBGPConfig(cfg.BGPConfig, verrs, pathPrefix+".bgpConfig")
		}
	}
}

func Validate_KubeVIPBGPConfig(cfg *KubeVIPBGPConfig, verrs *validation.ValidationErrors, path string) {
	if !helpers.IsValidNonEmptyString(cfg.RouterID) {
		verrs.Add(path+".routerID", "must be specified for BGP mode")
	} else if !helpers.IsValidIP(cfg.RouterID) {
		verrs.Add(path+".routerID", fmt.Sprintf("invalid IP address format '%s'", cfg.RouterID))
	}

	if cfg.ASN == 0 {
		verrs.Add(path+".asn", "local ASN must be a positive integer")
	}

	if cfg.PeerASN == 0 {
		verrs.Add(path+".peerASN", "peer ASN must be a positive integer")
	}

	if !helpers.IsValidNonEmptyString(cfg.PeerAddress) {
		verrs.Add(path+".peerAddress", "peer address must be specified for BGP mode")
	} else if !helpers.IsValidIP(cfg.PeerAddress) {
		verrs.Add(path+".peerAddress", fmt.Sprintf("invalid peer IP address format '%s'", cfg.PeerAddress))
	}
	if helpers.IsValidNonEmptyString(cfg.SourceAddress) && !helpers.IsValidIP(cfg.SourceAddress) {
		verrs.Add(path+".sourceAddress", fmt.Sprintf("invalid source IP address format '%s'", cfg.SourceAddress))
	}
}
