package v1alpha1

import (
	"strings"
	// Assuming ValidationErrors is in cluster_types.go or a shared util in this package
	// Assuming isValidIP and containsString are in cluster_types.go or a shared util
)

// DNS defines the overall DNS configuration for the cluster.
// Corresponds to `dns` in YAML.
type DNS struct {
	// DNSEtcHosts is a string containing custom entries to be merged into /etc/hosts on nodes/pods.
	// This allows for static host-to-IP mappings.
	// Corresponds to `dns.dnsEtcHosts` in YAML.
	DNSEtcHosts string `json:"dnsEtcHosts,omitempty" yaml:"dnsEtcHosts,omitempty"`

	// NodeEtcHosts is similar to DNSEtcHosts but specifically for the node's /etc/hosts file,
	// potentially differing from what's injected into pods. Optional.
	// Corresponds to `dns.nodeEtcHosts` in YAML.
	NodeEtcHosts string `json:"nodeEtcHosts,omitempty" yaml:"nodeEtcHosts,omitempty"`

	// CoreDNS configuration.
	// Corresponds to `dns.coredns` in YAML.
	CoreDNS *CoreDNS `json:"coredns,omitempty" yaml:"coredns,omitempty"` // Made pointer to be optional as a whole block

	// NodeLocalDNS configuration.
	// Corresponds to `dns.nodelocaldns` in YAML.
	NodeLocalDNS *NodeLocalDNS `json:"nodelocaldns,omitempty" yaml:"nodelocaldns,omitempty"` // Made pointer
}

// CoreDNS defines specific configurations for CoreDNS.
type CoreDNS struct {
	// AdditionalConfigs allows specifying raw Corefile snippets to be merged.
	// Corresponds to `dns.coredns.additionalConfigs` in YAML.
	AdditionalConfigs string `json:"additionalConfigs,omitempty" yaml:"additionalConfigs,omitempty"`

	// ExternalZones defines configurations for specific external DNS zones.
	// Corresponds to `dns.coredns.externalZones` in YAML.
	ExternalZones []ExternalZone `json:"externalZones,omitempty" yaml:"externalZones,omitempty"`

	// RewriteBlock allows specifying raw CoreDNS rewrite plugin configurations.
	// Corresponds to `dns.coredns.rewriteBlock` in YAML.
	RewriteBlock string `json:"rewriteBlock,omitempty" yaml:"rewriteBlock,omitempty"`

	// UpstreamDNSServers is a list of upstream DNS servers CoreDNS should forward queries to.
	// Corresponds to `dns.coredns.upstreamDNSServers` in YAML.
	UpstreamDNSServers []string `json:"upstreamDNSServers,omitempty" yaml:"upstreamDNSServers,omitempty"`
}

// NodeLocalDNS defines specific configurations for NodeLocal DNSCache.
type NodeLocalDNS struct {
	// ExternalZones defines configurations for specific external DNS zones for NodeLocalDNS.
	// Corresponds to `dns.nodelocaldns.externalZones` in YAML.
	ExternalZones []ExternalZone `json:"externalZones,omitempty" yaml:"externalZones,omitempty"`
	// Enabled indicates whether NodeLocalDNS should be deployed.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// ExternalZone defines rules for an external DNS zone.
type ExternalZone struct {
	// Zones is a list of domain names this configuration applies to.
	Zones []string `json:"zones" yaml:"zones"`
	// Nameservers are the authoritative DNS servers for these zones.
	Nameservers []string `json:"nameservers" yaml:"nameservers"`
	// Cache specifies the caching duration (in seconds) for records from these zones.
	Cache int `json:"cache,omitempty" yaml:"cache,omitempty"` // Cache TTL in seconds
	// Rewrite rules for queries matching these zones. (Format might need clarification or be raw strings)
	Rewrite []string `json:"rewrite,omitempty" yaml:"rewrite,omitempty"`
}


// SetDefaults_DNS sets default values for DNS config.
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

// SetDefaults_CoreDNS sets default values for CoreDNS config.
func SetDefaults_CoreDNS(cfg *CoreDNS) {
	if cfg == nil {
		return
	}
	if cfg.ExternalZones == nil {
		cfg.ExternalZones = []ExternalZone{}
	}
	for i := range cfg.ExternalZones { // Default cache for each zone if not set
		SetDefaults_ExternalZone(&cfg.ExternalZones[i])
	}
	if cfg.UpstreamDNSServers == nil {
		// Default to common public DNS servers if none provided.
		// These might come from common constants.
		cfg.UpstreamDNSServers = []string{"8.8.8.8", "1.1.1.1"}
	}
}

// SetDefaults_NodeLocalDNS sets default values for NodeLocalDNS config.
func SetDefaults_NodeLocalDNS(cfg *NodeLocalDNS) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		b := true // Default NodeLocalDNS to enabled
		cfg.Enabled = &b
	}
	if cfg.ExternalZones == nil {
		cfg.ExternalZones = []ExternalZone{}
	}
	for i := range cfg.ExternalZones {
		SetDefaults_ExternalZone(&cfg.ExternalZones[i])
	}
}

// SetDefaults_ExternalZone sets default values for an ExternalZone.
func SetDefaults_ExternalZone(cfg *ExternalZone) {
	if cfg == nil {
		return
	}
	if cfg.Zones == nil { cfg.Zones = []string{} }
	if cfg.Nameservers == nil { cfg.Nameservers = []string{} }
	if cfg.Cache == 0 { // 0 could mean "use CoreDNS default", or we set a specific default.
		cfg.Cache = 300 // Default to 5 minutes (300 seconds)
	}
	if cfg.Rewrite == nil { cfg.Rewrite = []string{} }
}

// Validate_DNS validates DNS configurations.
func Validate_DNS(cfg *DNS, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return // If entire DNS block is optional and not provided, nothing to validate.
	}
	// DNSEtcHosts and NodeEtcHosts are strings, specific validation for content might be complex.
	// Basic check: not just whitespace if set.
	if cfg.DNSEtcHosts != "" && strings.TrimSpace(cfg.DNSEtcHosts) == "" {
		verrs.Add(pathPrefix+".dnsEtcHosts", "cannot be only whitespace if specified")
	}
	if cfg.NodeEtcHosts != "" && strings.TrimSpace(cfg.NodeEtcHosts) == "" {
		verrs.Add(pathPrefix+".nodeEtcHosts", "cannot be only whitespace if specified")
	}

	if cfg.CoreDNS != nil {
		Validate_CoreDNS(cfg.CoreDNS, verrs, pathPrefix+".coredns")
	} else {
		// If CoreDNS is mandatory part of DNS config (even if empty struct for defaults)
		// verrs.Add(pathPrefix+".coredns", "CoreDNS configuration cannot be nil")
	}

	if cfg.NodeLocalDNS != nil {
		Validate_NodeLocalDNS(cfg.NodeLocalDNS, verrs, pathPrefix+".nodelocaldns")
	}
}

// Validate_CoreDNS validates CoreDNS configurations.
func Validate_CoreDNS(cfg *CoreDNS, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	for i, ez := range cfg.ExternalZones {
		Validate_ExternalZone(&ez, verrs, fmt.Sprintf("%s.externalZones[%d]", pathPrefix, i))
	}
	for i, upstream := range cfg.UpstreamDNSServers {
		if !isValidIP(upstream) { // Assuming isValidIP is available from cluster_types.go or util
			verrs.Add(fmt.Sprintf("%s.upstreamDNSServers[%d]", pathPrefix, i), "invalid IP address '%s'", upstream)
		}
	}
	// AdditionalConfigs and RewriteBlock are raw strings, complex to validate deeply here.
}

// Validate_NodeLocalDNS validates NodeLocalDNS configurations.
func Validate_NodeLocalDNS(cfg *NodeLocalDNS, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	// No specific validation for Enabled (*bool) other than type.
	for i, ez := range cfg.ExternalZones {
		Validate_ExternalZone(&ez, verrs, fmt.Sprintf("%s.externalZones[%d]", pathPrefix, i))
	}
}

// Validate_ExternalZone validates an ExternalZone configuration.
func Validate_ExternalZone(cfg *ExternalZone, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if len(cfg.Zones) == 0 {
		verrs.Add(pathPrefix+".zones", "must specify at least one zone")
	}
	for i, zone := range cfg.Zones {
		if strings.TrimSpace(zone) == "" {
			verrs.Add(fmt.Sprintf("%s.zones[%d]", pathPrefix, i), "zone name cannot be empty")
		}
		// Could add domain name validation for each zone.
	}
	if len(cfg.Nameservers) == 0 {
		verrs.Add(pathPrefix+".nameservers", "must specify at least one nameserver for external zone")
	}
	for i, ns := range cfg.Nameservers {
		if !isValidIP(ns) { // Assuming isValidIP
			verrs.Add(fmt.Sprintf("%s.nameservers[%d]", pathPrefix, i), "invalid IP address '%s' for nameserver", ns)
		}
	}
	if cfg.Cache < 0 {
		verrs.Add(pathPrefix+".cache", "cache TTL cannot be negative, got %d", cfg.Cache)
	}
	// Rewrite rules are strings, specific validation depends on expected format.
}

// NOTE: DeepCopy methods should be generated by controller-gen.
// Assumed ValidationErrors, isValidIP, containsString are available from cluster_types.go or a shared util.
// Made CoreDNS and NodeLocalDNS pointers in DNS struct to allow them to be optional blocks.
// Added Enabled field to NodeLocalDNS and its defaulting.
// Adjusted SetDefaults_CoreDNS to default UpstreamDNSServers if nil.
// Adjusted SetDefaults_ExternalZone for Cache.
// Added import "strings", "fmt". "net" for ParseCIDR (via isValidIP) might be needed in cluster_types.go.
