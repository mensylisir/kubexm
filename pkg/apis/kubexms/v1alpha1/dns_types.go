package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type DNS struct {
	DNSEtcHosts  string        `json:"dnsEtcHosts,omitempty" yaml:"dnsEtcHosts,omitempty"`
	NodeEtcHosts string        `json:"nodeEtcHosts,omitempty" yaml:"nodeEtcHosts,omitempty"`
	CoreDNS      *CoreDNS      `json:"coredns,omitempty" yaml:"coredns,omitempty"`
	NodeLocalDNS *NodeLocalDNS `json:"nodelocaldns,omitempty" yaml:"nodelocaldns,omitempty"`
}

type CoreDNS struct {
	AdditionalConfigs  string                    `json:"additionalConfigs,omitempty" yaml:"additionalConfigs,omitempty"`
	RewriteBlock       string                    `json:"rewriteBlock,omitempty" yaml:"rewriteBlock,omitempty"`
	UpstreamForwarding *UpstreamForwardingConfig `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	ExternalZones      []ExternalZone            `json:"externalZones,omitempty" yaml:"externalZones,omitempty"`
}

type UpstreamForwardingConfig struct {
	StaticServers     []string `json:"staticServers,omitempty" yaml:"staticServers,omitempty"`
	UseNodeResolvConf *bool    `json:"useNodeResolvConf,omitempty" yaml:"useNodeResolvConf,omitempty"`
	Policy            string   `json:"policy,omitempty" yaml:"policy,omitempty"`
	MaxConcurrent     *int     `json:"maxConcurrent,omitempty" yaml:"maxConcurrent,omitempty"`
}

type NodeLocalDNS struct {
	Enabled       *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	IP            string         `json:"ip,omitempty" yaml:"ip,omitempty"`
	ExternalZones []ExternalZone `json:"externalZones,omitempty" yaml:"externalZones,omitempty"`
}

type ExternalZone struct {
	Zones       []string      `json:"zones" yaml:"zones"`
	Nameservers []string      `json:"nameservers" yaml:"nameservers"`
	Cache       int           `json:"cache,omitempty" yaml:"cache,omitempty"`
	Rewrite     []RewriteRule `json:"rewrite,omitempty" yaml:"rewrite,omitempty"`
}

type RewriteRule struct {
	FromPattern string `json:"fromPattern" yaml:"fromPattern"`
	ToTemplate  string `json:"toTemplate" yaml:"toTemplate"`
}

func SetDefaults_DNS(cfg *DNS) {
	if cfg == nil {
		return
	}
	if cfg.CoreDNS == nil {
		cfg.CoreDNS = &CoreDNS{}
	}
	SetDefaults_CoreDNS(cfg.CoreDNS)

	if cfg.NodeLocalDNS == nil {
		cfg.NodeLocalDNS = &NodeLocalDNS{}
	}
	SetDefaults_NodeLocalDNS(cfg.NodeLocalDNS)
}

func SetDefaults_CoreDNS(cfg *CoreDNS) {
	if cfg == nil {
		return
	}

	if cfg.UpstreamForwarding == nil {
		cfg.UpstreamForwarding = &UpstreamForwardingConfig{}
	}
	SetDefaults_UpstreamForwardingConfig(cfg.UpstreamForwarding)

	if cfg.ExternalZones == nil {
		cfg.ExternalZones = []ExternalZone{}
	}
	for i := range cfg.ExternalZones {
		SetDefaults_ExternalZone(&cfg.ExternalZones[i])
	}
}

func SetDefaults_UpstreamForwardingConfig(cfg *UpstreamForwardingConfig) {
	if cfg == nil {
		return
	}
	if cfg.UseNodeResolvConf == nil {
		cfg.UseNodeResolvConf = helpers.BoolPtr(true)
	}
	if len(cfg.StaticServers) == 0 && !*cfg.UseNodeResolvConf {
		cfg.StaticServers = []string{common.DefaultCoreDNSUpstreamGoogle, common.DefaultCoreDNSUpstreamCloudflare}
	}
	if cfg.Policy == "" {
		cfg.Policy = common.UpstreamForwardingConfigRandom
	}
	if cfg.Policy == "" {
		cfg.Policy = common.UpstreamForwardingConfigRandom
	}
	if cfg.MaxConcurrent == nil {
		cfg.MaxConcurrent = helpers.IntPtr(common.DefaultMaxConcurrent)
	}
}

func SetDefaults_NodeLocalDNS(cfg *NodeLocalDNS) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(true)
	}
	if *cfg.Enabled && cfg.IP == "" {
		cfg.IP = common.DefaultLocalDNS
	}
	if cfg.ExternalZones == nil {
		cfg.ExternalZones = []ExternalZone{}
	}
	for i := range cfg.ExternalZones {
		SetDefaults_ExternalZone(&cfg.ExternalZones[i])
	}
}

func SetDefaults_ExternalZone(cfg *ExternalZone) {
	if cfg == nil {
		return
	}
	if cfg.Zones == nil {
		cfg.Zones = []string{}
	}
	if cfg.Nameservers == nil {
		cfg.Nameservers = []string{}
	}
	if cfg.Rewrite == nil {
		cfg.Rewrite = []RewriteRule{}
	}
	if cfg.Cache == 0 {
		cfg.Cache = common.DefaultExternalZoneCacheSeconds
	}
}

func Validate_DNS(cfg *DNS, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if cfg.DNSEtcHosts != "" && strings.TrimSpace(cfg.DNSEtcHosts) == "" {
		verrs.Add(p + ".dnsEtcHosts: cannot be only whitespace if specified")
	}
	if cfg.NodeEtcHosts != "" && strings.TrimSpace(cfg.NodeEtcHosts) == "" {
		verrs.Add(p + ".nodeEtcHosts: cannot be only whitespace if specified")
	}

	if cfg.CoreDNS != nil {
		Validate_CoreDNS(cfg.CoreDNS, verrs, path.Join(p, "coredns"))
	}

	if cfg.NodeLocalDNS != nil {
		Validate_NodeLocalDNS(cfg.NodeLocalDNS, verrs, path.Join(p, "nodelocaldns"))
	}
}

func Validate_CoreDNS(cfg *CoreDNS, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if cfg.UpstreamForwarding != nil {
		Validate_UpstreamForwardingConfig(cfg.UpstreamForwarding, verrs, path.Join(p, "upstream"))
	}

	for i, ez := range cfg.ExternalZones {
		Validate_ExternalZone(&ez, verrs, fmt.Sprintf("%s.externalZones[%d]", p, i))
	}
}

func Validate_UpstreamForwardingConfig(cfg *UpstreamForwardingConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	hasStatic := len(cfg.StaticServers) > 0
	useResolvConf := cfg.UseNodeResolvConf != nil && *cfg.UseNodeResolvConf
	if !hasStatic && !useResolvConf {
		verrs.Add(p + ": at least one static server or 'useNodeResolvConf: true' must be specified")
	}

	for i, upstream := range cfg.StaticServers {
		if !helpers.IsValidIP(upstream) {
			verrs.Add(fmt.Sprintf("%s.staticServers[%d]: invalid IP address '%s'", p, i, upstream))
		}
	}

	if cfg.Policy != "" && !helpers.ContainsStringWithEmpty(common.ValidUpstreamPolicies, cfg.Policy) {
		verrs.Add(fmt.Sprintf("%s.policy: invalid policy '%s', must be one of %v or empty",
			p, cfg.Policy, common.ValidUpstreamPolicies))
	}

	if cfg.MaxConcurrent != nil && *cfg.MaxConcurrent < 0 {
		verrs.Add(fmt.Sprintf("%s.maxConcurrent: cannot be negative, got %d", p, *cfg.MaxConcurrent))
	}
}

func Validate_NodeLocalDNS(cfg *NodeLocalDNS, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if cfg.Enabled != nil && *cfg.Enabled {
		for i, ez := range cfg.ExternalZones {
			Validate_ExternalZone(&ez, verrs, fmt.Sprintf("%s.externalZones[%d]", p, i))
		}
	}
}

func Validate_ExternalZone(cfg *ExternalZone, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if len(cfg.Zones) == 0 {
		verrs.Add(p + ".zones: must specify at least one zone")
	}
	for i, zone := range cfg.Zones {
		if strings.TrimSpace(zone) == "" {
			verrs.Add(fmt.Sprintf("%s.zones[%d]: zone name cannot be empty", p, i))
		} else if !helpers.IsValidDomainName(zone) {
			verrs.Add(fmt.Sprintf("%s.zones[%d]: invalid domain name format for zone '%s'", p, i, zone))
		}
	}

	if len(cfg.Nameservers) == 0 {
		verrs.Add(p + ".nameservers: must specify at least one nameserver")
	}
	for i, ns := range cfg.Nameservers {
		if !helpers.IsValidIP(ns) {
			verrs.Add(fmt.Sprintf("%s.nameservers[%d]: invalid IP address '%s'", p, i, ns))
		}
	}

	if cfg.Cache < 0 {
		verrs.Add(fmt.Sprintf("%s.cache: cannot be negative, got %d", p, cfg.Cache))
	}

	for i, rule := range cfg.Rewrite {
		rulePath := fmt.Sprintf("%s.rewrite[%d]", p, i)
		if strings.TrimSpace(rule.FromPattern) == "" {
			verrs.Add(rulePath + ".fromPattern: cannot be empty")
		}
		if strings.TrimSpace(rule.ToTemplate) == "" {
			verrs.Add(rulePath + ".toTemplate: cannot be empty")
		}
	}
}
