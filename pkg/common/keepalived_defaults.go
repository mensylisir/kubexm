package common

// Keepalived specific defaults.
const (
	DefaultKeepalivedAuthPass      = "kxm_pass"           // Default auth password for Keepalived.
	KeepalivedAuthTypePASS           = "PASS"                               // Default authentication type for Keepalived.
	KeepalivedAuthTypeAH             = "AH"                                 // AH authentication type for Keepalived.
	DefaultKeepalivedPreempt         = true                                 // Default preempt mode for Keepalived.
	DefaultKeepalivedCheckScript     = "/etc/keepalived/check_apiserver.sh" // Example health check script path for Keepalived.
	DefaultKeepalivedInterval        = 5                                    // Default health check interval for Keepalived.
	DefaultKeepalivedRise            = 2                                    // Default rise count for Keepalived health check.
	DefaultKeepalivedFall            = 2                                    // Default fall count for Keepalived health check.
	DefaultKeepalivedAdvertInt       = 1                                    // Default advertisement interval for Keepalived.
	DefaultKeepalivedLVScheduler     = "rr"                                 // Default LVS scheduler for Keepalived.
)
