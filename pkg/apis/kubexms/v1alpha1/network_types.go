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

type Network struct {
	Plugin          string           `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	KubePodsCIDR    string           `json:"kubePodsCIDR,omitempty" yaml:"kubePodsCIDR,omitempty"`
	KubeServiceCIDR string           `json:"kubeServiceCIDR,omitempty" yaml:"kubeServiceCIDR,omitempty"`
	Calico          *CalicoConfig    `json:"calico,omitempty" yaml:"calico,omitempty"`
	Cilium          *CiliumConfig    `json:"cilium,omitempty" yaml:"cilium,omitempty"`
	Flannel         *FlannelConfig   `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	KubeOvn         *KubeOvnConfig   `json:"kubeovn,omitempty" yaml:"kubeovn,omitempty"`
	Hybridnet       *HybridnetConfig `json:"hybridnet,omitempty" yaml:"hybridnet,omitempty"`
	Multus          *MultusConfig    `json:"multus,omitempty" yaml:"multus,omitempty"`
}

func SetDefaults_Network(cfg *Network) {
	if cfg == nil {
		return
	}

	if cfg.Plugin == "" {
		cfg.Plugin = string(common.CNITypeCalico)
	}

	switch cfg.Plugin {
	case string(common.CNITypeCalico):
		if cfg.Calico == nil {
			cfg.Calico = &CalicoConfig{}
		}
		SetDefaults_CalicoConfig(cfg.Calico)
	case string(common.CNITypeCilium):
		if cfg.Cilium == nil {
			cfg.Cilium = &CiliumConfig{}
		}
		SetDefaults_CiliumConfig(cfg.Cilium)
	case string(common.CNITypeFlannel):
		if cfg.Flannel == nil {
			cfg.Flannel = &FlannelConfig{}
		}
		SetDefaults_FlannelConfig(cfg.Flannel)
	case string(common.CNITypeKubeOvn):
		if cfg.KubeOvn == nil {
			cfg.KubeOvn = &KubeOvnConfig{}
		}
		SetDefaults_KubeOvnConfig(cfg.KubeOvn)
	case string(common.CNITypeHybridnet):
		if cfg.Hybridnet == nil {
			cfg.Hybridnet = &HybridnetConfig{}
		}
		SetDefaults_HybridnetConfig(cfg.Hybridnet)
	}

	if cfg.Multus == nil {
		cfg.Multus = &MultusConfig{}
	}
	SetDefaults_MultusConfig(cfg.Multus)

	if cfg.Plugin != string(common.CNITypeKubeOvn) {
		if cfg.KubeOvn == nil {
			cfg.KubeOvn = &KubeOvnConfig{}
		}
		SetDefaults_KubeOvnConfig(cfg.KubeOvn)
	}
	if cfg.Plugin != string(common.CNITypeHybridnet) {
		if cfg.Hybridnet == nil {
			cfg.Hybridnet = &HybridnetConfig{}
		}
		SetDefaults_HybridnetConfig(cfg.Hybridnet)
	}
}

func Validate_Network(cfg *Network, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add(pathPrefix + ": network configuration section cannot be nil")
		return
	}
	p := path.Join(pathPrefix)

	if strings.TrimSpace(cfg.KubePodsCIDR) == "" {
		verrs.Add(p + ".kubePodsCIDR: cannot be empty")
	} else {
		if !helpers.IsValidCIDR(cfg.KubePodsCIDR) {
			verrs.Add(fmt.Sprintf("%s.kubePodsCIDR: invalid CIDR format '%s'", p, cfg.KubePodsCIDR))
		}
	}

	if strings.TrimSpace(cfg.KubeServiceCIDR) == "" {
		verrs.Add(p + ".kubeServiceCIDR: cannot be empty")
	} else {
		if !helpers.IsValidCIDR(cfg.KubeServiceCIDR) {
			verrs.Add(fmt.Sprintf("%s.kubeServiceCIDR: invalid CIDR format '%s'", p, cfg.KubeServiceCIDR))
		}
	}

	if !helpers.ContainsStringWithEmpty(common.SupportedCNIPlugins, cfg.Plugin) {
		verrs.Add(fmt.Sprintf("%s.plugin: invalid plugin '%s', must be one of [%s] or empty for default",
			p, cfg.Plugin, strings.Join(common.SupportedCNIPlugins, ", ")))
	}

	if _, podsNet, podsErr := net.ParseCIDR(cfg.KubePodsCIDR); podsErr == nil {
		if _, serviceNet, serviceErr := net.ParseCIDR(cfg.KubeServiceCIDR); serviceErr == nil {
			if podsNet.Contains(serviceNet.IP) || serviceNet.Contains(podsNet.IP) {
				verrs.Add(fmt.Sprintf("%s: kubePodsCIDR (%s) and kubeServiceCIDR (%s) must not overlap",
					p, cfg.KubePodsCIDR, cfg.KubeServiceCIDR))
			}
		}
	}
	switch cfg.Plugin {
	case string(common.CNITypeCalico):
		if cfg.Calico == nil {
			verrs.Add(p + ".calico: config section cannot be empty if plugin is 'calico'")
		} else {
			Validate_CalicoConfig(cfg.Calico, verrs, path.Join(p, "calico"))
		}
	case string(common.CNITypeCilium):
		if cfg.Cilium == nil {
			verrs.Add(p + ".cilium: config section cannot be empty if plugin is 'cilium'")
		} else {
			Validate_CiliumConfig(cfg.Cilium, verrs, path.Join(p, "cilium"))
		}
	case string(common.CNITypeFlannel):
		if cfg.Flannel == nil {
			verrs.Add(p + ".flannel: config section cannot be empty if plugin is 'flannel'")
		} else {
			Validate_FlannelConfig(cfg.Flannel, verrs, path.Join(p, "flannel"))
		}
	case string(common.CNITypeKubeOvn):
		if cfg.KubeOvn == nil {
			verrs.Add(p + ".kubeovn: config section cannot be empty if plugin is 'kube-ovn'")
		} else {
			Validate_KubeOvnConfig(cfg.KubeOvn, verrs, path.Join(p, "kubeovn"))
		}
	case string(common.CNITypeHybridnet):
		if cfg.Hybridnet == nil {
			verrs.Add(p + ".hybridnet: config section cannot be empty if plugin is 'hybridnet'")
		} else {
			Validate_HybridnetConfig(cfg.Hybridnet, verrs, path.Join(p, "hybridnet"))
		}
	}

	if cfg.Multus != nil {
		Validate_MultusConfig(cfg.Multus, verrs, path.Join(p, "multus"))
	}

	if cfg.Plugin != string(common.CNITypeKubeOvn) && cfg.KubeOvn != nil {
		Validate_KubeOvnConfig(cfg.KubeOvn, verrs, path.Join(p, "kubeovn"))
	}
	if cfg.Plugin != string(common.CNITypeHybridnet) && cfg.Hybridnet != nil {
		Validate_HybridnetConfig(cfg.Hybridnet, verrs, path.Join(p, "hybridnet"))
	}
}
