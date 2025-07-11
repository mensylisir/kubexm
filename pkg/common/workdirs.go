package common

// Default working directory names for Kubexm.
// These are typically combined with base paths defined in paths.go or user configuration.
const (
	// DefaultRemoteWorkDir is the default base working directory on remote target hosts.
	DefaultRemoteWorkDir = "/tmp/kubexms_work"

	// DefaultWorkDirName is the default name for the main Kubexm work directory on the control machine
	// (often within the execution path, e.g., $(pwd)/.kubexm, or under a global work_dir).
	// This is also used as the cluster-specific subdirectory name under the global work directory.
	DefaultWorkDirName = ".kubexm" // This was also KubexmRootDirName in paths.go, consider consolidating if truly identical.

	// DefaultTmpDirName is the default name for a temporary directory used by Kubexm
	// on the control machine, typically created under the system's temporary path (e.g., /tmp/.kubexm_tmp).
	DefaultTmpDirName = ".kubexm_tmp"
)
