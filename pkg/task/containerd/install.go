package containerd

import (
	"fmt"
	"path/filepath" // For joining paths

	"github.com/mensylisir/kubexm/pkg/spec"
	// Import specific step spec packages - assuming these now return spec.StepSpec
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/component_downloads"
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
	// "github.com/mensylisir/kubexm/pkg/config" // Example: if params like cfg *config.Cluster are needed
)

const (
	// Cache key constants for this task (can be used by steps to coordinate)
	cacheKeyContainerdDownloadedPath = "InstallContainerdTask.ContainerdDownloadedPath"
	cacheKeyContainerdExtractedPath  = "InstallContainerdTask.ContainerdExtractedPath"
	// Default download directory on target host, relative to a base work dir.
	// The actual base work dir (e.g. global work dir from runtime) should be prepended by the Executor
	// or steps themselves if they have access to it. For TaskSpec, these are relative paths or placeholders.
	defaultContainerdDownloadSubDir = "downloads/containerd"
	defaultContainerdExtractSubDir  = "extract/containerd"
)

// NewInstallContainerdTaskSpec creates a new TaskSpec for installing and configuring containerd.
// Parameters:
//   version: The version of containerd to install.
//   arch: Target architecture (e.g., "amd64", "arm64"). If empty, steps might auto-detect or use a default.
//   zone: Download zone/mirror.
//   downloadDir: Base directory for downloads on the target host. If empty, a default will be used.
//                This path might be interpreted relative to a global work directory by the executor/steps.
//   checksum: Expected checksum of the downloaded archive.
//   registryMirrors: Map of registry mirrors for containerd configuration.
//   insecureRegistries: List of insecure registries for containerd configuration.
//   useSystemdCgroup: Whether to configure containerd to use systemd cgroup driver.
//   extraTomlContent: Additional TOML content to append to the containerd config.
//   containerdConfigPath: Path to the containerd config.toml file on the target host.
//   runOnRoles: A slice of strings specifying which host roles this task should run on.
//   globalWorkDir: Base directory for work operations on target hosts (e.g., /opt/kubexm/clusterXYZ).
//                  This is used here to construct full paths for download/extraction if not overridden.
func NewInstallContainerdTaskSpec(
	version string, arch string, zone string, downloadDir string, checksum string,
	registryMirrors map[string]string, insecureRegistries []string,
	useSystemdCgroup bool, extraTomlContent string, containerdConfigPath string,
	runOnRoles []string,
	globalWorkDir string, // Added to resolve paths, though ideally steps handle this via context
) *spec.TaskSpec {

	// Determine dynamic paths. These are intended to be paths on the *target* hosts.
	// The Executor or the steps themselves will need to ensure these paths are correctly resolved.
	// For now, we construct them here based on globalWorkDir.
	hostSpecificDownloadDir := downloadDir
	if hostSpecificDownloadDir == "" {
		hostSpecificDownloadDir = filepath.Join(globalWorkDir, defaultContainerdDownloadSubDir)
	}
	hostSpecificExtractDir := filepath.Join(globalWorkDir, defaultContainerdExtractSubDir, version)

	taskSteps := []spec.StepSpec{
		component_downloads.NewDownloadContainerdStep( // Assuming this returns spec.StepSpec
			version, arch, zone, hostSpecificDownloadDir, checksum,
			cacheKeyContainerdDownloadedPath, // OutputFilePathKey
			"",                               // OutputFileNameKey
			"",                               // OutputComponentTypeKey
			"",                               // OutputVersionKey
			"",                               // OutputArchKey
			"",                               // OutputChecksumKey
			"",                               // OutputURLKey
		),
		common.NewExtractArchiveStep( // Assuming this returns spec.StepSpec
			cacheKeyContainerdDownloadedPath, // InputArchiveKey
			hostSpecificExtractDir,           // DestDir
			cacheKeyContainerdExtractedPath,  // OutputDirKey
			"",                               // ArchiveType (auto-detect)
			false,                            // StripComponents
			true,                             // Overwrite
		),
		stepContainerd.NewInstallContainerdStep( // Assuming this returns spec.StepSpec
			cacheKeyContainerdExtractedPath, // ExtractedPathKey
			nil,                             // FilesToInstall (map, optional, defaults in step)
			"",                              // InstallPrefix (optional, defaults in step)
			"",                              // ServiceFilePathKey (output, optional)
			"",                              // BinaryDirKey (output, optional)
		),
		stepContainerd.NewConfigureContainerdStep( // Assuming this returns spec.StepSpec
			registryMirrors,
			insecureRegistries,
			containerdConfigPath, // If empty, step uses default
			extraTomlContent,
			useSystemdCgroup,
			"", // OutputConfigPathKey (optional)
		),
		// Manage Service Steps
		stepContainerd.NewManageContainerdServiceStep(stepContainerd.ServiceActionDaemonReload, ""),
		stepContainerd.NewManageContainerdServiceStep(stepContainerd.ServiceActionEnable, ""),
		stepContainerd.NewManageContainerdServiceStep(stepContainerd.ServiceActionStart, ""),
	}

	return &spec.TaskSpec{
		Name:        "InstallAndConfigureContainerd",
		Description: fmt.Sprintf("Installs and configures containerd version %s", version),
		RunOnRoles:  runOnRoles,
		Steps:       taskSteps,
		IgnoreError: false,
		// Filter: "", // No specific filter string defined for this task yet
		// Concurrency: 0, // Use global default
	}
}
