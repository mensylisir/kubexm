package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"net"
	"path"
	"strings"
)

type Extra struct {
	EtcHosts   []EtcHostEntry    `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	ResolvConf *ResolvConfConfig `json:"resolves,omitempty" yaml:"resolves,omitempty"`
}

type EtcHostEntry struct {
	IP        string   `json:"ip" yaml:"ip"`
	Hostnames []string `json:"hostnames" yaml:"hostnames"`
}

type ResolvConfConfig struct {
	Nameservers []string `json:"nameservers,omitempty" yaml:"nameservers,omitempty"`
	Searches    []string `json:"searches,omitempty" yaml:"searches,omitempty"`
	Options     []string `json:"options,omitempty" yaml:"options,omitempty"`
}

func SetDefaults_Extra(cfg *Extra) {
	if cfg == nil {
		return
	}

	if cfg.EtcHosts == nil {
		cfg.EtcHosts = []EtcHostEntry{}
	}
	for i := range cfg.EtcHosts {
		if cfg.EtcHosts[i].Hostnames == nil {
			cfg.EtcHosts[i].Hostnames = []string{}
		}
	}

	if cfg.ResolvConf != nil {
		if cfg.ResolvConf.Nameservers == nil {
			cfg.ResolvConf.Nameservers = []string{}
		}
		if cfg.ResolvConf.Searches == nil {
			cfg.ResolvConf.Searches = []string{}
		}
		if cfg.ResolvConf.Options == nil {
			cfg.ResolvConf.Options = []string{}
		}
	}
}

func Validate_Extra(cfg *Extra, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	// 校验 EtcHosts
	for i, entry := range cfg.EtcHosts {
		entryPath := fmt.Sprintf("%s.etcHosts[%d]", p, i)
		Validate_EtcHostEntry(&entry, verrs, entryPath)
	}

	if cfg.ResolvConf != nil {
		Validate_ResolvConfConfig(cfg.ResolvConf, verrs, path.Join(p, "resolvConf"))
	}
}

func Validate_EtcHostEntry(cfg *EtcHostEntry, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if strings.TrimSpace(cfg.IP) == "" {
		verrs.Add(pathPrefix + ".ip: cannot be empty")
	} else if net.ParseIP(cfg.IP) == nil {
		verrs.Add(fmt.Sprintf("%s.ip: invalid IP address format for '%s'", pathPrefix, cfg.IP))
	}

	if len(cfg.Hostnames) == 0 {
		verrs.Add(pathPrefix + ".hostnames: must contain at least one hostname")
	}
	for i, hostname := range cfg.Hostnames {
		if strings.TrimSpace(hostname) == "" {
			verrs.Add(fmt.Sprintf("%s.hostnames[%d]: hostname cannot be empty", pathPrefix, i))
		}
	}
}

func Validate_ResolvConfConfig(cfg *ResolvConfConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, ns := range cfg.Nameservers {
		if strings.TrimSpace(ns) == "" {
			verrs.Add(fmt.Sprintf("%s.nameservers[%d]: nameserver cannot be empty", pathPrefix, i))
		} else if net.ParseIP(ns) == nil {
			verrs.Add(fmt.Sprintf("%s.nameservers[%d]: invalid IP address format for '%s'", pathPrefix, i, ns))
		}
	}

	for i, search := range cfg.Searches {
		if strings.TrimSpace(search) == "" {
			verrs.Add(fmt.Sprintf("%s.searches[%d]: search domain cannot be empty", pathPrefix, i))
		} else if !helpers.IsValidDomainName(search) {
			verrs.Add(fmt.Sprintf("%s.searches[%d]: invalid domain name format for '%s'", pathPrefix, i, search))
		}
	}
}
