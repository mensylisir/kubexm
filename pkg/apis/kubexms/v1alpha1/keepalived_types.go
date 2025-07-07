package v1alpha1

import (
	"fmt"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

const (
	// KeepalivedAuthTypePass represents the PASS authentication type for Keepalived.
	KeepalivedAuthTypePass = "PASS"
	// KeepalivedAuthTypeAH represents the AH (Authentication Header) type for Keepalived.
	KeepalivedAuthTypeAH = "AH"
)

var (
	// validKeepalivedAuthTypes lists the supported authentication types for Keepalived.
	validKeepalivedAuthTypes = []string{KeepalivedAuthTypePass, KeepalivedAuthTypeAH}
)

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
func Validate_KeepalivedConfig(cfg *KeepalivedConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.VRID == nil {
		verrs.Add(pathPrefix+".vrid", "virtual router ID must be specified")
	} else if *cfg.VRID < 1 || *cfg.VRID > 255 {
		verrs.Add(pathPrefix+".vrid", fmt.Sprintf("must be between 1 and 255, got %d", *cfg.VRID))
	}

	if cfg.Priority == nil {
		verrs.Add(pathPrefix+".priority", "must be specified for master/backup election")
	} else if *cfg.Priority < 1 || *cfg.Priority > 254 {
		verrs.Add(pathPrefix+".priority", fmt.Sprintf("must be between 1 and 254, got %d", *cfg.Priority))
	}

	if cfg.Interface == nil || strings.TrimSpace(*cfg.Interface) == "" {
		verrs.Add(pathPrefix+".interface", "network interface must be specified")
	}

	// AuthType validation
	if cfg.AuthType == nil { // defensive check, though SetDefaults should prevent this
		verrs.Add(pathPrefix+".authType", "is required and should have a default value 'PASS'")
	} else { // AuthType is not nil, proceed with validation
		if !containsString(validKeepalivedAuthTypes, *cfg.AuthType) {
			verrs.Add(pathPrefix+".authType", fmt.Sprintf("invalid value '%s', must be one of %v", *cfg.AuthType, validKeepalivedAuthTypes))
		}

		// AuthPass validation based on AuthType
		if *cfg.AuthType == KeepalivedAuthTypePass {
			if cfg.AuthPass == nil || strings.TrimSpace(*cfg.AuthPass) == "" {
				verrs.Add(pathPrefix+".authPass", "must be specified if authType is 'PASS'")
			} else if len(*cfg.AuthPass) > 8 {
				verrs.Add(pathPrefix+".authPass", "password too long, ensure compatibility (max 8 chars for some versions)")
			}
		} else if *cfg.AuthType == KeepalivedAuthTypeAH { // AuthType is known to be non-nil here
			if cfg.AuthPass != nil && *cfg.AuthPass != "" {
				verrs.Add(pathPrefix+".authPass", "should not be specified if authType is 'AH'")
			}
		}
	}

	for i, line := range cfg.ExtraConfig {
	   if strings.TrimSpace(line) == "" {
		   verrs.Add(fmt.Sprintf("%s.extraConfig[%d]", pathPrefix, i), "extra config line cannot be empty")
	   }
	}
}
