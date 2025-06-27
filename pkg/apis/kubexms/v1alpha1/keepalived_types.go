package v1alpha1

import "strings"

// KeepalivedConfig defines settings for Keepalived service used for HA.
type KeepalivedConfig struct {
	// VRID is the Virtual Router ID, must be unique in the network segment.
	// Range: 0-255.
	VRID *int `json:"vrid,omitempty" yaml:"vrid,omitempty"`

	// Priority determines master/backup election. Higher value means higher priority.
	// Range: 1-254. Masters usually have higher values (e.g., 101) than backups (e.g., 100).
	Priority *int `json:"priority,omitempty" yaml:"priority,omitempty"`

	// Interface is the network interface Keepalived should bind to for VRRP.
	// Example: "eth0", "ens192".
	Interface *string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// AuthType specifies the authentication method for VRRP.
	// Supported: "PASS", "AH". Defaults to "PASS".
	AuthType *string `json:"authType,omitempty" yaml:"authType,omitempty"`

	// AuthPass is the password for "PASS" authentication type.
	// Required if AuthType is "PASS". Max 8 characters for older keepalived versions.
	AuthPass *string `json:"authPass,omitempty" yaml:"authPass,omitempty"`

	// ExtraConfig allows adding raw lines to the keepalived.conf.
	// Each string is a line to be appended.
	ExtraConfig []string `json:"extraConfig,omitempty" yaml:"extraConfig,omitempty"`

	// SkipInstall, if true, assumes Keepalived is already installed and configured externally.
	// KubeXMS will then only use the VIP information if provided in HAConfig.
	SkipInstall *bool `json:"skipInstall,omitempty" yaml:"skipInstall,omitempty"`
}

// --- Defaulting Functions ---

// SetDefaults_KeepalivedConfig sets default values for KeepalivedConfig.
func SetDefaults_KeepalivedConfig(cfg *KeepalivedConfig) {
	if cfg == nil {
		return
	}
	if cfg.AuthType == nil {
		cfg.AuthType = stringPtr("PASS")
	}
	if cfg.SkipInstall == nil {
		cfg.SkipInstall = boolPtr(false) // Default to managing keepalived installation
	}
	if cfg.ExtraConfig == nil {
		cfg.ExtraConfig = []string{}
	}
	// VRID, Priority, Interface, AuthPass are highly environment-specific,
	// so no strong universal defaults here. They should be set by user or
	// intelligently derived during a planning phase if possible.
	// For example, Priority might be defaulted differently for master vs backup nodes.
	// For now, validation will catch their absence if they are required.
}

// --- Validation Functions ---

// Validate_KeepalivedConfig validates KeepalivedConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
func Validate_KeepalivedConfig(cfg *KeepalivedConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return // Nothing to validate if the config section is absent
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return // If skipping install, most other fields are not KubeXMS's concern to validate for setup.
	}

	if cfg.VRID == nil {
		verrs.Add("%s.vrid: virtual router ID must be specified", pathPrefix)
	} else if *cfg.VRID < 0 || *cfg.VRID > 255 {
		verrs.Add("%s.vrid: must be between 0 and 255, got %d", pathPrefix, *cfg.VRID)
	}

	if cfg.Priority == nil {
		verrs.Add("%s.priority: must be specified for master/backup election", pathPrefix)
	} else if *cfg.Priority < 1 || *cfg.Priority > 254 { // 0 and 255 are reserved by VRRP spec for master with VIP owner and preemption disable
		verrs.Add("%s.priority: must be between 1 and 254, got %d", pathPrefix, *cfg.Priority)
	}

	if cfg.Interface == nil || strings.TrimSpace(*cfg.Interface) == "" {
		verrs.Add("%s.interface: network interface must be specified", pathPrefix)
	}

	// AuthType is defaulted to "PASS", so cfg.AuthType will not be nil if defaults were applied.
	validAuthTypes := []string{"PASS", "AH"}
	if !containsString(validAuthTypes, *cfg.AuthType) {
		verrs.Add("%s.authType: invalid value '%s', must be one of %v", pathPrefix, *cfg.AuthType, validAuthTypes)
	}

	if *cfg.AuthType == "PASS" {
		if cfg.AuthPass == nil || strings.TrimSpace(*cfg.AuthPass) == "" {
			verrs.Add("%s.authPass: must be specified if authType is 'PASS'", pathPrefix)
		} else if len(*cfg.AuthPass) > 8 {
			// Older Keepalived versions have an 8-character limit for password.
			// Newer versions might support longer. This is a conservative check.
			verrs.Add("%s.authPass: password too long, ensure compatibility (max 8 chars for some versions)", pathPrefix)
		}
	}
	if cfg.AuthType != nil && *cfg.AuthType == "AH" && cfg.AuthPass != nil && *cfg.AuthPass != "" {
		verrs.Add("%s.authPass: should not be specified if authType is 'AH'", pathPrefix)
	}

	for i, line := range cfg.ExtraConfig {
	   if strings.TrimSpace(line) == "" { // Disallow empty lines if they could break config
		   verrs.Add("%s.extraConfig[%d]: extra config line cannot be empty", pathPrefix, i)
	   }
	}
}
