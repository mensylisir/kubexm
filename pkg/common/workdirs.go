package common

// Default working directory names for Kubexm.
// These are typically combined with base paths defined in paths.go or user configuration.
const (
	// DefaultRemoteWorkDirTarget is the default base working directory on remote target hosts.
	DefaultRemoteWorkDirTarget = "/tmp/kubexms_work"

	// DefaultTmpDirNameLocal is the default name for a temporary directory used by Kubexm
	// on the control machine, typically created under the system's temporary path (e.g., /tmp/.kubexm_tmp).
	DefaultTmpDirNameLocal = ".kubexm_tmp"
)

// Note: DefaultWorkDirName and KubexmRootDirName are now consolidated in types.go as DefaultLocalWorkDir
