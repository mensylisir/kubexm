package v1alpha1

import (
	"fmt"
	"strings"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util"
)

var (
	// validKeepalivedAuthTypes lists the supported authentication types for Keepalived.
	validKeepalivedAuthTypes = []string{common.KeepalivedAuthTypePASS, common.KeepalivedAuthTypeAH}
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

	// Preempt allows a higher priority machine to take over from a lower priority one.
	// Defaults to true.
	Preempt *bool `json:"preempt,omitempty" yaml:"preempt,omitempty"`
	// CheckScript is the path to a script that Keepalived will run to check service health.
	CheckScript *string `json:"checkScript,omitempty" yaml:"checkScript,omitempty"`
	// Interval is the health check interval in seconds.
	Interval *int `json:"interval,omitempty" yaml:"interval,omitempty"`
	// Rise is the number of successful checks required to transition to MASTER state.
	Rise *int `json:"rise,omitempty" yaml:"rise,omitempty"`
	// Fall is the number of failed checks required to transition to BACKUP/FAULT state.
	Fall *int `json:"fall,omitempty" yaml:"fall,omitempty"`
	// AdvertInt is the VRRP advertisement interval in seconds.
	AdvertInt *int `json:"advertInt,omitempty" yaml:"advertInt,omitempty"`
	// LVScheduler is the LVS scheduling algorithm (e.g., "rr", "wrr", "lc", "wlc").
	// Used if Keepalived is managing LVS.
	LVScheduler *string `json:"lvScheduler,omitempty" yaml:"lvScheduler,omitempty"`
}

// --- Defaulting Functions ---

// SetDefaults_KeepalivedConfig sets default values for KeepalivedConfig.
func SetDefaults_KeepalivedConfig(cfg *KeepalivedConfig) {
	if cfg == nil {
		return
	}
	if cfg.AuthType == nil {
		cfg.AuthType = util.StrPtr(common.KeepalivedAuthTypePASS)
	}
	if cfg.AuthType != nil && *cfg.AuthType == common.KeepalivedAuthTypePASS && cfg.AuthPass == nil {
		cfg.AuthPass = util.StrPtr(common.DefaultKeepalivedAuthPass)
	}

	if cfg.SkipInstall == nil {
		cfg.SkipInstall = util.BoolPtr(false) // Default to managing keepalived installation
	}
	if cfg.ExtraConfig == nil {
		cfg.ExtraConfig = []string{}
	}

	if cfg.Preempt == nil {
		cfg.Preempt = util.BoolPtr(common.DefaultKeepalivedPreempt)
	}
	if cfg.CheckScript == nil {
		cfg.CheckScript = util.StrPtr(common.DefaultKeepalivedCheckScript)
	}
	if cfg.Interval == nil {
		cfg.Interval = util.IntPtr(common.DefaultKeepalivedInterval)
	}
	if cfg.Rise == nil {
		cfg.Rise = util.IntPtr(common.DefaultKeepalivedRise)
	}
	if cfg.Fall == nil {
		cfg.Fall = util.IntPtr(common.DefaultKeepalivedFall)
	}
	if cfg.AdvertInt == nil {
		cfg.AdvertInt = util.IntPtr(common.DefaultKeepalivedAdvertInt)
	}
	if cfg.LVScheduler == nil {
		cfg.LVScheduler = util.StrPtr(common.DefaultKeepalivedLVScheduler)
	}

	// VRID and Priority are often context-dependent (e.g. master vs backup)
	// No universal defaults here, but can be set by higher-level logic if needed.
	// Interface is also very environment specific.
}

// --- Validation Functions ---

// Validate_KeepalivedConfig validates KeepalivedConfig.
func Validate_KeepalivedConfig(cfg *KeepalivedConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.VRID == nil {
		verrs.Add(pathPrefix+".vrid", "virtual router ID must be specified")
	} else if *cfg.VRID < 1 || *cfg.VRID > 255 {
		verrs.Add(pathPrefix + ".vrid: must be between 1 and 255, got " + fmt.Sprintf("%d", *cfg.VRID))
	}

	if cfg.Priority == nil {
		verrs.Add(pathPrefix+".priority", "must be specified for master/backup election")
	} else if *cfg.Priority < 1 || *cfg.Priority > 254 {
		verrs.Add(pathPrefix + ".priority: must be between 1 and 254, got " + fmt.Sprintf("%d", *cfg.Priority))
	}

	if cfg.Interface == nil || strings.TrimSpace(*cfg.Interface) == "" {
		verrs.Add(pathPrefix+".interface", "network interface must be specified")
	}

	// AuthType validation
	if cfg.AuthType == nil { // defensive check, though SetDefaults should prevent this
		verrs.Add(pathPrefix+".authType", "is required and should have a default value 'PASS'")
	} else { // AuthType is not nil, proceed with validation
		if !util.ContainsString(validKeepalivedAuthTypes, *cfg.AuthType) { // Use util.ContainsString
			verrs.Add(pathPrefix + ".authType: invalid value '" + *cfg.AuthType + "', must be one of " + fmt.Sprintf("%v", validKeepalivedAuthTypes))
		}

		// AuthPass validation based on AuthType
		if *cfg.AuthType == common.KeepalivedAuthTypePASS { // Use common.KeepalivedAuthTypePASS
			if cfg.AuthPass == nil || strings.TrimSpace(*cfg.AuthPass) == "" {
				verrs.Add(pathPrefix+".authPass", "must be specified if authType is 'PASS'")
			} else if len(*cfg.AuthPass) > 8 {
				verrs.Add(pathPrefix+".authPass", "password too long, ensure compatibility (max 8 chars for some versions)")
			}
		} else if *cfg.AuthType == common.KeepalivedAuthTypeAH { // AuthType is known to be non-nil here. Use common.KeepalivedAuthTypeAH
			if cfg.AuthPass != nil && *cfg.AuthPass != "" {
				verrs.Add(pathPrefix+".authPass", "should not be specified if authType is 'AH'")
			}
		}
	}

	for i, line := range cfg.ExtraConfig {
	   if strings.TrimSpace(line) == "" {
		   verrs.Add(fmt.Sprintf("%s.extraConfig[%d]: extra config line cannot be empty", pathPrefix, i))
	   }
	}

	if cfg.CheckScript != nil && strings.TrimSpace(*cfg.CheckScript) == "" {
		verrs.Add(pathPrefix+".checkScript", "cannot be empty if specified")
	}
	if cfg.Interval != nil && *cfg.Interval <= 0 {
		verrs.Add(pathPrefix + ".interval: must be positive if specified, got " + fmt.Sprintf("%d", *cfg.Interval))
	}
	if cfg.Rise != nil && *cfg.Rise <= 0 {
		verrs.Add(pathPrefix + ".rise: must be positive if specified, got " + fmt.Sprintf("%d", *cfg.Rise))
	}
	if cfg.Fall != nil && *cfg.Fall <= 0 {
		verrs.Add(pathPrefix + ".fall: must be positive if specified, got " + fmt.Sprintf("%d", *cfg.Fall))
	}
	if cfg.AdvertInt != nil && *cfg.AdvertInt <= 0 {
		verrs.Add(pathPrefix + ".advertInt: must be positive if specified, got " + fmt.Sprintf("%d", *cfg.AdvertInt))
	}
	if cfg.LVScheduler != nil {
		if strings.TrimSpace(*cfg.LVScheduler) == "" {
			verrs.Add(pathPrefix+".lvScheduler", "cannot be empty if specified")
		}
		// Optional: Validate against a list of known LVS schedulers if desired
		// validLVSchedulers := []string{"rr", "wrr", "lc", "wlc", "lblc", "sh", "dh"}
		// if !containsString(validLVSchedulers, *cfg.LVScheduler) {
		// 	verrs.Add(pathPrefix+".lvScheduler", "invalid LVS scheduler '%s'", *cfg.LVScheduler)
		// }
	}
}
