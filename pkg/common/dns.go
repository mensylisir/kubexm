package common

// DomainValidationRegexString is used for validating domain names.
const DomainValidationRegexString = `^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`

// DNS specific defaults.
const (
	DefaultCoreDNSUpstreamGoogle     = "8.8.8.8" // Google's public DNS.
	DefaultCoreDNSUpstreamCloudflare = "1.1.1.1" // Cloudflare's public DNS.
	DefaultExternalZoneCacheSeconds  = 300       // Default cache time for external DNS zones.
)
