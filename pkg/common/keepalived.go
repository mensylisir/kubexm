package common

// Keepalived specific defaults.
const (
	DefaultKeepalivedAuthPass                  = "kxm_pass"                           // Default auth password for Keepalived.
	KeepalivedAuthTypePASS                     = "PASS"                               // Default authentication type for Keepalived.
	KeepalivedAuthTypeAH                       = "AH"                                 // AH authentication type for Keepalived.
	DefaultKeepalivedPreempt                   = true                                 // Default preempt mode for Keepalived.
	DefaultKeepalivedCheckScript               = "/etc/keepalived/check_apiserver.sh" // Example health check script path for Keepalived.
	DefaultKeepalivedInterval                  = 5                                    // Default health check interval for Keepalived.
	DefaultKeepalivedRise                      = 2                                    // Default rise count for Keepalived health check.
	DefaultKeepalivedFall                      = 2                                    // Default fall count for Keepalived health check.
	DefaultKeepalivedAdvertInt                 = 1                                    // Default advertisement interval for Keepalived.
	DefaultKeepalivedWeight                    = 1
	DefaultKeepalivedTimepout                  = 3
	DefaultKeepalivedRetries                   = 3
	DefaultKeepalivedRetriesInterval           = 3
	DefaultKeepalivedHTTPCheckSucessStatusCode = 200
	DefaultKeepalivedLVSRRcheduler             = "rr"
	DefaultKeepalivedLVSWRRcheduler            = "wrr"
	DefaultKeepalivedLVSLCcheduler             = "lc"
	DefaultKeepalivedLVSWLCcheduler            = "wlc"
	DefaultKeepalivedLVSLBLCcheduler           = "lblc"
	DefaultKeepalivedLVSSHcheduler             = "sh"
	DefaultKeepalivedLVSDHcheduler             = "dh"
	DefaultKeepalivedDR                        = "DR"
	DefaultKeepalivedNAT                       = "NAT"
	DefaultKeepalivedTUN                       = "TUN"
	DefaultKeepalivedTCPProtocol               = "TCP"
	DefaultKeepalivedUDPProtocol               = "UDP"
	DefaultKeepalivedVRID                      = 51  // Default Virtual Router ID for Keepalived.
	DefaultKeepalivedPriorityMaster            = 110 // Default priority for master node in Keepalived.
	DefaultKeepalivedRouterID                  = "LVS_DEVEL"
	DefaultKeepaliveMaster                     = "MASTER"
	DefaultKeepaliveBackup                     = "BACKUP"
)

const (
	KeepalivedDefaultConfDirTarget    = "/etc/keepalived"
	KeepalivedDefaultConfigFileTarget = "/etc/keepalived/keepalived.conf"
	KeepalivedDefaultSystemdFile      = "/etc/systemd/system/keepalived.service"
	DefaultKeepalivedConfig           = "keepalived.conf"
)
