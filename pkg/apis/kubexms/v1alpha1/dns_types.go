package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
)

// DNS defines the overall DNS configuration for the cluster.
// It includes settings for host-level DNS overrides, CoreDNS, and NodeLocalDNS.
type DNS struct {
	// DNSEtcHosts allows specifying custom entries to be added to /etc/hosts for Pods.
	// Format is a multi-line string, with each line being a standard /etc/hosts entry (e.g., "1.2.3.4 myhost.example.com myhost").
	DNSEtcHosts string `yaml:"dnsEtcHosts" json:"dnsEtcHosts"`
	// NodeEtcHosts allows specifying custom entries to be added to /etc/hosts on the nodes themselves.
	// Format is the same as DNSEtcHosts. Optional.
	NodeEtcHosts string `yaml:"nodeEtcHosts" json:"nodeEtcHosts,omitempty"`
	// CoreDNS holds the configuration for the CoreDNS addon.
	CoreDNS CoreDNS `yaml:"coredns" json:"coredns"`
	// NodeLocalDNS holds the configuration for the NodeLocal DNSCache addon.
	NodeLocalDNS NodeLocalDNS `yaml:"nodelocaldns" json:"nodelocaldns"`
}

// CoreDNS defines the configuration for the CoreDNS addon.
type CoreDNS struct {
	// AdditionalConfigs allows injecting raw CoreDNS Corefile snippets into the main configuration.
	// This can be used for advanced configurations not directly exposed by other fields.
	AdditionalConfigs string `yaml:"additionalConfigs" json:"additionalConfigs"`
	// ExternalZones defines custom upstream DNS servers for specific external domains.
	ExternalZones []ExternalZone `yaml:"externalZones" json:"externalZones"`
	// RewriteBlock allows injecting raw CoreDNS rewrite rules.
	// Example: "rewrite name foo.example.com bar.example.com"
	RewriteBlock string `yaml:"rewriteBlock" json:"rewriteBlock"`
	// UpstreamDNSServers is a list of upstream DNS servers that CoreDNS will forward queries to.
	// These are used for domains not covered by ExternalZones or other specific configurations.
	// Example: ["8.8.8.8", "1.1.1.1"]
	UpstreamDNSServers []string `yaml:"upstreamDNSServers" json:"upstreamDNSServers"`
}

// NodeLocalDNS defines the configuration for the NodeLocal DNSCache addon.
// NodeLocal DNSCache improves cluster DNS performance by running a dns caching agent on cluster nodes.
type NodeLocalDNS struct {
	// ExternalZones defines custom upstream DNS servers for specific external domains for the NodeLocal DNSCache.
	// This allows NodeLocal DNSCache to forward queries for these zones to designated resolvers.
	ExternalZones []ExternalZone `yaml:"externalZones" json:"externalZones"`
}

// ExternalZone defines a custom DNS forwarding rule for a set of specified domain zones.
type ExternalZone struct {
	// Zones is a list of domain names (or suffixes) for which this external zone rule applies.
	// Example: ["example.com", "internal.net"]
	Zones []string `yaml:"zones" json:"zones"`
	// Nameservers is a list of IP addresses of upstream DNS servers to forward queries for the specified Zones.
	// Example: ["10.0.0.1", "10.0.0.2"]
	Nameservers []string `yaml:"nameservers" json:"nameservers"`
	// Cache specifies the DNS caching time (in seconds) for records resolved through this external zone.
	// Defaults to 300 seconds if not set or set to 0 by SetDefaults_ExternalZone.
	Cache int `yaml:"cache" json:"cache"`
	// Rewrite defines a list of rewrite rules to be applied for queries matching this external zone.
	// Example: {"fromPattern": "(.*)\\.example\\.com", "toTemplate": "{1}.my-internal-domain.com"}
	Rewrite []RewriteRule `yaml:"rewrite" json:"rewrite,omitempty"`
}

// RewriteRule defines a DNS rewrite rule.
// It specifies a pattern to match DNS query names and a template to rewrite them.
type RewriteRule struct {
	// FromPattern is a regular expression used to match the DNS query name.
	// Capture groups (e.g., (.*)) can be used here and referenced in ToTemplate.
	// Example: "(.*)\\.partner\\.example\\.com"
	FromPattern string `json:"fromPattern" yaml:"fromPattern"`
	// ToTemplate is the template string for the rewritten DNS query name.
	// It can use capture groups from FromPattern, denoted as {N} where N is the capture group index (e.g., {1}, {2}).
	// Example: "{1}.internal.example.local"
	ToTemplate string `json:"toTemplate" yaml:"toTemplate"`
}

// SetDefaults_DNS sets default values for DNS configuration.
func SetDefaults_DNS(cfg *DNS) {
	if cfg == nil {
		return
	}

	// Defaults for CoreDNS
	// No explicit default for cfg.CoreDNS itself, assume if DNS struct is present, CoreDNS part can be too.
	// If user provides coredns: {} then cfg.CoreDNS will be non-nil.
	if cfg.CoreDNS.UpstreamDNSServers == nil {
		cfg.CoreDNS.UpstreamDNSServers = []string{"8.8.8.8", "1.1.1.1"} // Common public DNS
	}
	if cfg.CoreDNS.ExternalZones == nil {
		cfg.CoreDNS.ExternalZones = []ExternalZone{}
	}
	for i := range cfg.CoreDNS.ExternalZones {
		SetDefaults_ExternalZone(&cfg.CoreDNS.ExternalZones[i])
	}

	// Defaults for NodeLocalDNS
	if cfg.NodeLocalDNS.ExternalZones == nil {
		cfg.NodeLocalDNS.ExternalZones = []ExternalZone{}
	}
	for i := range cfg.NodeLocalDNS.ExternalZones {
		SetDefaults_ExternalZone(&cfg.NodeLocalDNS.ExternalZones[i])
	}
	// DNSEtcHosts and NodeEtcHosts are strings, typically no complex defaults needed unless specific content is expected.
}

// SetDefaults_ExternalZone sets default values for an ExternalZone.
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
	if cfg.Cache == 0 { // Assuming 0 means not set, default to a sensible value like 300s
		cfg.Cache = 300
	}
	if cfg.Rewrite == nil {
		cfg.Rewrite = []RewriteRule{}
	}
}

// Validate_DNS validates the DNS configuration.
func Validate_DNS(cfg *DNS, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		// If the entire DNS section is optional and not provided, this is fine.
		// If it's required, the caller (Validate_Cluster) should check for cfg != nil.
		return
	}

	// Validate CoreDNS
	coreDNSPath := pathPrefix + ".coredns"
	if cfg.CoreDNS.UpstreamDNSServers != nil {
		if len(cfg.CoreDNS.UpstreamDNSServers) == 0 {
			// Allow empty if user explicitly provides empty list, otherwise default would fill it.
			// Or enforce at least one if that's a requirement. For now, allow empty if set.
		}
		for i, server := range cfg.CoreDNS.UpstreamDNSServers {
			if strings.TrimSpace(server) == "" {
				verrs.Add("%s.upstreamDNSServers[%d]: server address cannot be empty", coreDNSPath, i)
			} else if !util.ValidateHostPortString(server) && !util.IsValidIP(server) && !util.IsValidDomainName(server) {
				// Try ValidateHostPortString first, if that fails, it might be a simple IP or Domain without port
				verrs.Add("%s.upstreamDNSServers[%d]: invalid server address format '%s'", coreDNSPath, i, server)
			}
		}
	}
	for i, ez := range cfg.CoreDNS.ExternalZones {
		ezPath := fmt.Sprintf("%s.externalZones[%d]", coreDNSPath, i)
		Validate_ExternalZone(&ez, verrs, ezPath)
	}
	// AdditionalConfigs and RewriteBlock are free-form strings, harder to validate structurally.

	// Validate NodeLocalDNS
	nodeLocalDNSPath := pathPrefix + ".nodelocaldns"
	for i, ez := range cfg.NodeLocalDNS.ExternalZones {
		ezPath := fmt.Sprintf("%s.externalZones[%d]", nodeLocalDNSPath, i)
		Validate_ExternalZone(&ez, verrs, ezPath)
	}
	// DNSEtcHosts and NodeEtcHosts are strings. Could validate for non-whitespace if set and not empty.
	if cfg.DNSEtcHosts != "" && strings.TrimSpace(cfg.DNSEtcHosts) == "" {
		verrs.Add("%s.dnsEtcHosts: cannot be only whitespace if specified", pathPrefix)
	}
	if cfg.NodeEtcHosts != "" && strings.TrimSpace(cfg.NodeEtcHosts) == "" {
		verrs.Add("%s.nodeEtcHosts: cannot be only whitespace if specified", pathPrefix)
	}

}

// Validate_ExternalZone validates an ExternalZone configuration.
func Validate_ExternalZone(cfg *ExternalZone, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if len(cfg.Zones) == 0 {
		verrs.Add("%s.zones: must contain at least one zone name", pathPrefix)
	}
	for i, zone := range cfg.Zones {
		if strings.TrimSpace(zone) == "" {
			verrs.Add("%s.zones[%d]: zone name cannot be empty", pathPrefix, i)
		} else if !util.IsValidDomainName(zone) { // Use util.IsValidDomainName
			verrs.Add("%s.zones[%d]: invalid domain name format '%s'", pathPrefix, i, zone)
		}
	}

	if len(cfg.Nameservers) == 0 {
		verrs.Add("%s.nameservers: must contain at least one nameserver", pathPrefix)
	}
	for i, ns := range cfg.Nameservers {
		if strings.TrimSpace(ns) == "" {
			verrs.Add("%s.nameservers[%d]: nameserver address cannot be empty", pathPrefix, i)
		} else if !util.ValidateHostPortString(ns) && !util.IsValidIP(ns) && !util.IsValidDomainName(ns) {
			// Try ValidateHostPortString first, if that fails, it might be a simple IP or Domain without port
			verrs.Add("%s.nameservers[%d]: invalid nameserver address format '%s'", pathPrefix, i, ns)
		}
	}

	if cfg.Cache < 0 {
		verrs.Add("%s.cache: cannot be negative, got %d", pathPrefix, cfg.Cache)
	}

	for i, rule := range cfg.Rewrite {
		rulePath := fmt.Sprintf("%s.rewrite[%d]", pathPrefix, i)
		if strings.TrimSpace(rule.FromPattern) == "" {
			verrs.Add("%s.fromPattern: cannot be empty", rulePath)
		}
		if strings.TrimSpace(rule.ToTemplate) == "" {
			verrs.Add("%s.toTemplate: cannot be empty", rulePath)
		}
		// Further regex validation for FromPattern could be added if necessary,
		// but that might be too complex for this level of validation.
	}
}
