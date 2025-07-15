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

type CalicoConfig struct {
	Networking         *CalicoNetworking         `json:"networking,omitempty" yaml:"networking,omitempty"`
	TyphaDeployment    *CalicoTyphaDeployment    `json:"typha,omitempty" yaml:"typha,omitempty"` // Renamed from TyphaDeployment to just Typha
	IPAM               *CalicoIPAM               `json:"ipam,omitempty" yaml:"ipam,omitempty"`
	FelixConfiguration *CalicoFelixConfiguration `json:"felix,omitempty" yaml:"felix,omitempty"`
}

type CalicoNetworking struct {
	IPIPMode         string                  `json:"ipipMode,omitempty" yaml:"ipipMode,omitempty"`
	VXLANMode        string                  `json:"vxlanMode,omitempty" yaml:"vxlanMode,omitempty"`
	VethMTU          *int                    `json:"vethMTU,omitempty" yaml:"vethMTU,omitempty"`
	BGPConfiguration *CalicoBGPConfiguration `json:"bgp,omitempty" yaml:"bgp,omitempty"`
}

type CalicoBGPConfiguration struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type CalicoTyphaDeployment struct {
	Enabled      *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Replicas     *int              `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
}

type CalicoIPAM struct {
	Pools           []CalicoIPPool `json:"pools,omitempty" yaml:"pools,omitempty"`
	AutoCreatePools *bool          `json:"autoCreatePools,omitempty" yaml:"autoCreatePools,omitempty"` // Renamed from DefaultIPPOOL
}

type CalicoIPPool struct {
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	CIDR          string `json:"cidr" yaml:"cidr"`
	Encapsulation string `json:"encapsulation,omitempty" yaml:"encapsulation,omitempty"`
	NatOutgoing   *bool  `json:"natOutgoing,omitempty" yaml:"natOutgoing,omitempty"`
	BlockSize     *int   `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
	Disabled      *bool  `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

type CalicoFelixConfiguration struct {
	LogSeverityScreen string `json:"logSeverityScreen,omitempty"yaml:"logSeverityScreen,omitempty"`
}

func SetDefaults_CalicoConfig(cfg *CalicoConfig) {
	if cfg == nil {
		return
	}

	if cfg.Networking == nil {
		cfg.Networking = &CalicoNetworking{}
	}
	if cfg.Networking.IPIPMode == "" {
		cfg.Networking.IPIPMode = common.DefaultCalicoIPIPMode
	}
	if cfg.Networking.VXLANMode == "" {
		cfg.Networking.VXLANMode = common.DefaultCalicoVXLANMode
	}
	if cfg.Networking.BGPConfiguration == nil {
		cfg.Networking.BGPConfiguration = &CalicoBGPConfiguration{}
	}
	if cfg.Networking.BGPConfiguration.Enabled == nil {
		cfg.Networking.BGPConfiguration.Enabled = helpers.BoolPtr(common.CalicoBGPConfigurationEnable)
	}

	if cfg.TyphaDeployment == nil {
		cfg.TyphaDeployment = &CalicoTyphaDeployment{}
	}
	if cfg.TyphaDeployment.Enabled == nil {
		cfg.TyphaDeployment.Enabled = helpers.BoolPtr(common.CalicoTyphaDeploymentEnable)
	}
	if cfg.TyphaDeployment.Replicas == nil {
		cfg.TyphaDeployment.Replicas = helpers.IntPtr(common.CalicoTyphaDeploymentReplicas)
	}

	if cfg.IPAM == nil {
		cfg.IPAM = &CalicoIPAM{}
	}
	if cfg.IPAM.AutoCreatePools == nil {
		cfg.IPAM.AutoCreatePools = helpers.BoolPtr(common.CalicoIPAMAutoCreatePools)
	}
	for i := range cfg.IPAM.Pools {
		pool := &cfg.IPAM.Pools[i]
		if pool.NatOutgoing == nil {
			pool.NatOutgoing = helpers.BoolPtr(common.CalicoIPPoolNatOutgoing)
		}
		if pool.BlockSize == nil {
			pool.BlockSize = helpers.IntPtr(common.DefaultCalicoIPPoolBlockSize)
		}
		if pool.Disabled == nil {
			pool.Disabled = helpers.BoolPtr(common.DefaultCalicoIPPoolDisabled)
		}
	}

	if cfg.FelixConfiguration == nil {
		cfg.FelixConfiguration = &CalicoFelixConfiguration{}
	}
	if cfg.FelixConfiguration.LogSeverityScreen == "" {
		cfg.FelixConfiguration.LogSeverityScreen = common.DefaultCalicoFelixConfigurationLogSeverityScreen
	}
}

func Validate_CalicoConfig(cfg *CalicoConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Networking != nil {
		p := path.Join(pathPrefix, "networking")
		if !helpers.ContainsStringWithEmpty(common.ValidCalicoEncapsulationModes, cfg.Networking.IPIPMode) {
			verrs.Add(fmt.Sprintf("%s.ipipMode: invalid mode '%s', must be one of [%s] or empty",
				p, cfg.Networking.IPIPMode, strings.Join(common.ValidCalicoEncapsulationModes, ", ")))
		}
		if !helpers.ContainsStringWithEmpty(common.ValidCalicoEncapsulationModes, cfg.Networking.VXLANMode) {
			verrs.Add(fmt.Sprintf("%s.vxlanMode: invalid mode '%s', must be one of [%s] or empty",
				p, cfg.Networking.VXLANMode, strings.Join(common.ValidCalicoEncapsulationModes, ", ")))
		}
		if cfg.Networking.VethMTU != nil && (*cfg.Networking.VethMTU < 60 || *cfg.Networking.VethMTU > 65535) {
			verrs.Add(fmt.Sprintf("%s.vethMTU: invalid MTU %d, must be between 60 and 65535", p, *cfg.Networking.VethMTU))
		}
	}

	if cfg.TyphaDeployment != nil {
		p := path.Join(pathPrefix, "typha")
		if cfg.TyphaDeployment.Replicas != nil && *cfg.TyphaDeployment.Replicas < 0 {
			verrs.Add(fmt.Sprintf("%s.replicas: cannot be negative, got %d", p, *cfg.TyphaDeployment.Replicas))
		}
	}

	if cfg.IPAM != nil {
		p := path.Join(pathPrefix, "ipam", "pools")
		for i, pool := range cfg.IPAM.Pools {
			poolPath := fmt.Sprintf("%s[%d]", p, i)

			if pool.CIDR == "" {
				verrs.Add(fmt.Sprintf("%s.cidr: is a required field", poolPath))
			} else {
				if _, _, err := net.ParseCIDR(pool.CIDR); err != nil {
					verrs.Add(fmt.Sprintf("%s.cidr: invalid CIDR format for '%s': %v", poolPath, pool.CIDR, err))
				}
			}

			if pool.Encapsulation != "" && !helpers.ContainsString(common.ValidCalicoIPPoolEncapsulationModes, pool.Encapsulation) {
				verrs.Add(fmt.Sprintf("%s.encapsulation: invalid mode '%s', must be one of [%s]",
					poolPath, pool.Encapsulation, strings.Join(common.ValidCalicoIPPoolEncapsulationModes, ", ")))
			}

			if pool.BlockSize != nil && (*pool.BlockSize < 20 || *pool.BlockSize > 32) {
				verrs.Add(fmt.Sprintf("%s.blockSize: invalid size %d, must be between 20 and 32", poolPath, *pool.BlockSize))
			}
		}
	}

	if cfg.FelixConfiguration != nil {
		p := path.Join(pathPrefix, "felix")
		if !helpers.ContainsStringWithEmpty(common.ValidCalicoLogSeverities, cfg.FelixConfiguration.LogSeverityScreen) {
			verrs.Add(fmt.Sprintf("%s.logSeverityScreen: invalid level '%s', must be one of [%s] or empty",
				p, cfg.FelixConfiguration.LogSeverityScreen, strings.Join(common.ValidCalicoLogSeverities, ", ")))
		}
	}
}
