package common

// Default working directory names for Kubexm.
// These are typically combined with base paths defined in paths.go or user configuration.
const (
	// DefaultRemoteWorkDir is the default base working directory on remote target hosts.
	DefaultRemoteWorkDir = "/tmp/kubexms_work"

	// DefaultTmpDirName is the default name for a temporary directory used by Kubexm
	// on the control machine, typically created under the system's temporary path (e.g., /tmp/.kubexm_tmp).
	DefaultTmpDirName = ".kubexm_tmp"
)

// Note: DefaultWorkDirName and KubexmRootDirName are now consolidated in types.go as DefaultLocalWorkDir
