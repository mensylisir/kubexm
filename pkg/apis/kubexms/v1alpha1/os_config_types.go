package v1alpha1

import "strings"

// OSConfig defines OS-level configurations for hosts in the cluster.
// This includes settings like NTP, timezone, and prerequisite packages.
type OSConfig struct {
	// NtpServers is a list of NTP server addresses to configure on the hosts.
	NtpServers []string `json:"ntpServers,omitempty"`

	// Timezone to set on the hosts, e.g., "Etc/UTC", "America/New_York".
	Timezone *string `json:"timezone,omitempty"`

	// Rpms is a list of RPM package names to ensure are installed.
	// Primarily for RPM-based systems.
	Rpms []string `json:"rpms,omitempty"`

	// Debs is a list of DEB package names to ensure are installed.
	// Primarily for Debian-based systems.
	Debs []string `json:"debs,omitempty"`

	// SkipConfigureOS, if true, skips OS configuration steps like setting NTP or timezone.
	// Defaults to false.
	SkipConfigureOS *bool `json:"skipConfigureOS,omitempty"`
}

// --- Defaulting Functions ---

// SetDefaults_OSConfig sets default values for OSConfig.
func SetDefaults_OSConfig(cfg *OSConfig) {
	if cfg == nil {
		return
	}
	if cfg.NtpServers == nil {
		cfg.NtpServers = []string{} // Default to empty, user must specify or OS default used
	}
	// No default for Timezone, let OS default prevail if not set.
	if cfg.Rpms == nil {
		cfg.Rpms = []string{}
	}
	if cfg.Debs == nil {
		cfg.Debs = []string{}
	}
	if cfg.SkipConfigureOS == nil {
		b := false // Default to performing OS configuration
		cfg.SkipConfigureOS = &b
	}
}

// --- Validation Functions ---

// Validate_OSConfig validates OSConfig.
func Validate_OSConfig(cfg *OSConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, ntp := range cfg.NtpServers {
		if strings.TrimSpace(ntp) == "" {
			verrs.Add("%s.ntpServers[%d]: NTP server address cannot be empty", pathPrefix, i)
		}
		// Could add validation for hostname/IP format for NTP servers
	}
	if cfg.Timezone != nil && strings.TrimSpace(*cfg.Timezone) == "" {
		verrs.Add("%s.timezone: cannot be empty if specified", pathPrefix)
		// Could validate against a list of known timezones if necessary (complex)
	}
	for i, rpm := range cfg.Rpms {
		if strings.TrimSpace(rpm) == "" {
			verrs.Add("%s.rpms[%d]: RPM package name cannot be empty", pathPrefix, i)
		}
	}
	for i, deb := range cfg.Debs {
		if strings.TrimSpace(deb) == "" {
			verrs.Add("%s.debs[%d]: DEB package name cannot be empty", pathPrefix, i)
		}
	}
}
