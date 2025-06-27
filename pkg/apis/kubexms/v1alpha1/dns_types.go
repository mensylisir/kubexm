package v1alpha1

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
