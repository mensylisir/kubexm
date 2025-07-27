package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type FlannelConfig struct {
	Source  AddonSource           `json:"source,omitempty" yaml:"sources,omitempty"`
	Backend *FlannelBackendConfig `json:"backend,omitempty" yaml:"backend,omitempty"`
}

type FlannelBackendConfig struct {
	Type   string               `json:"type,omitempty" yaml:"type,omitempty"`
	VXLAN  *FlannelVXLANConfig  `json:"vxlan,omitempty" yaml:"vxlan,omitempty"`
	HostGW *FlannelHostGWConfig `json:"host-gw,omitempty" yaml:"host-gw,omitempty"`
	IPsec  *FlannelIPsecConfig  `json:"ipsec,omitempty" yaml:"ipsec,omitempty"`
}

type FlannelVXLANConfig struct {
	VNI           *int  `json:"vni,omitempty" yaml:"vni,omitempty"`
	Port          *int  `json:"port,omitempty" yaml:"port,omitempty"`
	DirectRouting *bool `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`
}

type FlannelHostGWConfig struct {
	DirectRouting *bool `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`
}

type FlannelIPsecConfig struct {
	PSKSecretName string `json:"pskSecretName,omitempty" yaml:"pskSecretName,omitempty"`
}

func SetDefaults_FlannelConfig(cfg *FlannelConfig) {
	if cfg == nil {
		return
	}

	if cfg.Backend == nil {
		cfg.Backend = &FlannelBackendConfig{}
	}

	if cfg.Backend.Type == "" {
		cfg.Backend.Type = common.DefaultFlannelBackendConfigType
	}

	if cfg.Backend.Type == common.DefaultFlannelBackendConfigType {
		if cfg.Backend.VXLAN == nil {
			cfg.Backend.VXLAN = &FlannelVXLANConfig{}
		}
		if cfg.Backend.VXLAN.VNI == nil {
			cfg.Backend.VXLAN.VNI = helpers.IntPtr(common.DefaultFlannelVXLANConfigVNI)
		}
		if cfg.Backend.VXLAN.Port == nil {
			cfg.Backend.VXLAN.Port = helpers.IntPtr(common.DefaultFlannelVXLANConfigPort)
		}
		if cfg.Backend.VXLAN.DirectRouting == nil {
			cfg.Backend.VXLAN.DirectRouting = helpers.BoolPtr(common.DefaultFlannelVXLANConfigDirectRouting)
		}
	}
}

func Validate_FlannelConfig(cfg *FlannelConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Backend == nil {
		return
	}
	p := path.Join(pathPrefix, "backend")

	if !helpers.ContainsStringWithEmpty(common.ValidFlannelBackendTypes, cfg.Backend.Type) {
		verrs.Add(fmt.Sprintf("%s.type: invalid type '%s', must be one of [%s] or empty",
			p, cfg.Backend.Type, strings.Join(common.ValidFlannelBackendTypes, ", ")))
	}

	if cfg.Backend.Type != common.FlannelBackendConfigTypeVxlan && cfg.Backend.VXLAN != nil {
		verrs.Add(fmt.Sprintf("%s.vxlan: can only be set when backend type is 'vxlan'", p))
	}

	if cfg.Backend.Type != common.FlannelBackendConfigTypeHostGw && cfg.Backend.HostGW != nil {
		verrs.Add(fmt.Sprintf("%s.host-gw: can only be set when backend type is 'host-gw'", p))
	}

	if cfg.Backend.Type != common.FlannelBackendConfigTypeIpsec && cfg.Backend.IPsec != nil {
		verrs.Add(fmt.Sprintf("%s.ipsec: can only be set when backend type is 'ipsec'", p))
	}

	if cfg.Backend.VXLAN != nil {
		vxlanPath := path.Join(p, "vxlan")
		if cfg.Backend.VXLAN.Port != nil && (*cfg.Backend.VXLAN.Port <= 0 || *cfg.Backend.VXLAN.Port > 65535) {
			verrs.Add(fmt.Sprintf("%s.port: invalid port %d, must be between 1-65535", vxlanPath, *cfg.Backend.VXLAN.Port))
		}
	}
}
