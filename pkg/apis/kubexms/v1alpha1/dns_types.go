package v1alpha1

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// isValidHostOrIP checks if a string is a valid IP address or a simple valid hostname.
// For hostnames, this is a simplified check.
func isValidHostOrIP(hostOrIP string) bool {
	if net.ParseIP(hostOrIP) != nil {
		return true // It's a valid IP address
	}
	// Regex for basic hostname validation (allows LDH labels: letters, digits, hyphen)
	// Does not enforce all RFC rules (e.g. length of parts, total length) but good for most cases.
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, hostOrIP); matched {
		return true
	}
	return false
}

// isValidDomainName checks if a string is a plausible domain name.
// This is a simplified check.
func isValidDomainName(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	// Regex for basic domain name validation
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, domain); !matched {
		return false
	}
	// Should not end with a hyphen or dot
	if strings.HasSuffix(domain, "-") || strings.HasSuffix(domain, ".") {
		return false
	}
	// Should not start with a hyphen
	if strings.HasPrefix(domain, "-") {
		return false
	}
	return true
}


type DNS struct {
	DNSEtcHosts  string       `yaml:"dnsEtcHosts" json:"dnsEtcHosts"`
	NodeEtcHosts string       `yaml:"nodeEtcHosts" json:"nodeEtcHosts,omitempty"`
	CoreDNS      CoreDNS      `yaml:"coredns" json:"coredns"`
	NodeLocalDNS NodeLocalDNS `yaml:"nodelocaldns" json:"nodelocaldns"`
}

type CoreDNS struct {
	AdditionalConfigs  string         `yaml:"additionalConfigs" json:"additionalConfigs"`
	ExternalZones      []ExternalZone `yaml:"externalZones" json:"externalZones"`
	RewriteBlock       string         `yaml:"rewriteBlock" json:"rewriteBlock"`
	UpstreamDNSServers []string       `yaml:"upstreamDNSServers" json:"upstreamDNSServers"`
}

type NodeLocalDNS struct {
	ExternalZones []ExternalZone `yaml:"externalZones" json:"externalZones"`
}

type ExternalZone struct {
	Zones       []string `yaml:"zones" json:"zones"`
	Nameservers []string `yaml:"nameservers" json:"nameservers"`
	Cache       int      `yaml:"cache" json:"cache"`
	Rewrite     []string `yaml:"rewrite" json:"rewrite"`
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
		cfg.Rewrite = []string{}
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
			} else if !isValidHostOrIP(server) {
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
		} else if !isValidDomainName(zone) {
			verrs.Add("%s.zones[%d]: invalid domain name format '%s'", pathPrefix, i, zone)
		}
	}

	if len(cfg.Nameservers) == 0 {
		verrs.Add("%s.nameservers: must contain at least one nameserver", pathPrefix)
	}
	for i, ns := range cfg.Nameservers {
		if strings.TrimSpace(ns) == "" {
			verrs.Add("%s.nameservers[%d]: nameserver address cannot be empty", pathPrefix, i)
		} else if !isValidHostOrIP(ns) {
			verrs.Add("%s.nameservers[%d]: invalid nameserver address format '%s'", pathPrefix, i, ns)
		}
	}

	if cfg.Cache < 0 {
		verrs.Add("%s.cache: cannot be negative, got %d", pathPrefix, cfg.Cache)
	}

	for i, rw := range cfg.Rewrite {
		if strings.TrimSpace(rw) == "" {
			verrs.Add("%s.rewrite[%d]: rewrite rule cannot be empty", pathPrefix, i)
		}
		// More complex validation for rewrite rule syntax could be added if known.
	}
}
