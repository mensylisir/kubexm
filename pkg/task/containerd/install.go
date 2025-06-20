package containerd

import (
	"fmt"
	"path/filepath" // For joining paths for temporary extraction

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host in Plan
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task" // Import task package for task.Interface and task.BaseTask

	// Import step packages
	// "github.com/mensylisir/kubexm/pkg/step" // For step.Step interface type (not directly used here as concrete steps are)
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/component_downloads"
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
)

const (
	// Cache key constants for this task
	cacheKeyContainerdDownloadedPath = "InstallContainerdTask.ContainerdDownloadedPath" // Made more specific
	cacheKeyContainerdExtractedPath  = "InstallContainerdTask.ContainerdExtractedPath"  // Made more specific
	// Default download directory on target host, relative to global work dir
	defaultContainerdDownloadSubDir = "downloads/containerd"
	defaultContainerdExtractSubDir  = "extract/containerd"    // Subdir within host's global work dir portion
)

// InstallContainerdTask installs and configures containerd.
type InstallContainerdTask struct {
	*task.BaseTask
	Version              string
	Arch                 string // Optional: if empty, step might auto-detect
	Zone                 string // Optional: for download mirrors
	DownloadDir          string // Optional: if empty, a default under host's work dir will be used
	Checksum             string // Optional: checksum for the downloaded archive
	RegistryMirrors      map[string]string
	InsecureRegistries   []string
	UseSystemdCgroup     bool
	ExtraTomlContent     string
	ContainerdConfigPath string // Optional: defaults in ConfigureContainerdStep
}

// NewInstallContainerdTask creates a new task for installing and configuring containerd.
func NewInstallContainerdTask(
	version string, arch string, zone string, downloadDir string, checksum string,
	registryMirrors map[string]string, insecureRegistries []string,
	useSystemdCgroup bool, extraToml string, configPath string,
	runOnRoles []string,
) task.Interface {
	base := task.NewBaseTask(
		"InstallAndConfigureContainerd",
		fmt.Sprintf("Installs and configures containerd version %s", version),
		runOnRoles,
		nil,
		false,
	)
	return &InstallContainerdTask{
		BaseTask:             &base,
		Version:              version,
		Arch:                 arch,
		Zone:                 zone,
		DownloadDir:          downloadDir,
		Checksum:             checksum,
		RegistryMirrors:      registryMirrors,
		InsecureRegistries:   insecureRegistries,
		UseSystemdCgroup:     useSystemdCgroup,
		ExtraTomlContent:     extraToml,
		ContainerdConfigPath: configPath,
	}
}

// Description is inherited from BaseTask.
// func (t *InstallContainerdTask) Description() string { return t.BaseTask.TaskDesc }

// IsRequired is inherited from BaseTask (defaults to true if RunOnRoles is empty, or if hosts match RunOnRoles).
// func (t *InstallContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) { return t.BaseTask.IsRequired(ctx) }


// Plan generates the execution plan to install and configure containerd.
func (t *InstallContainerdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	execPlan := &plan.ExecutionPlan{Phases: []plan.Phase{}}

	// 1. Determine target hosts
	var targetHosts []connector.Host
	if len(t.BaseTask.RunOnRoles) == 0 {
		logger.Info("No specific roles defined for InstallContainerdTask. This task will not target any hosts by role.")
		// If the intent is to run on ALL hosts when no roles are specified,
		// then ctx.GetAllHosts() or similar would be needed.
		// For now, empty roles = no hosts by this logic.
		return execPlan, nil
	}
	for _, role := range t.BaseTask.RunOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			return nil, fmt.Errorf("failed to get hosts for role '%s' in task %s: %w", role, t.Name(), err)
		}
		targetHosts = append(targetHosts, hosts...)
	}

	uniqueHostsMap := make(map[string]connector.Host)
	for _, h := range targetHosts {
		uniqueHostsMap[h.GetName()] = h
	}
	targetHosts = []connector.Host{}
	for _, h := range uniqueHostsMap {
		targetHosts = append(targetHosts, h)
	}

	if len(targetHosts) == 0 {
		logger.Info("No target hosts found for InstallContainerdTask after role filtering. Returning empty plan.")
		return execPlan, nil
	}

	hostNamesForLog := []string{}
	for _, h := range targetHosts { hostNamesForLog = append(hostNamesForLog, h.GetName()) }
	logger.Infof("Planning containerd installation for %d hosts: %v", len(targetHosts), hostNamesForLog)


	// Determine dynamic paths using global work dir. These paths are on the *target* hosts.
	// The actual directory structure within GlobalWorkDir (e.g., per-host subdirectories)
	// should be handled consistently by the steps or by how GlobalWorkDir is constructed/used.
	// For simplicity, we assume steps can work with a base path and create necessary sub-structs.

	// DownloadDir for the actual download step on the target host
    hostSpecificDownloadDir := t.DownloadDir
    if hostSpecificDownloadDir == "" {
        // This default path is relative to where the step will operate on the target host.
        // If GlobalWorkDir is like "/opt/kubexm/clusterX", then this becomes "/opt/kubexm/clusterX/downloads/containerd"
        // The download step itself should create this if it doesn't exist.
        hostSpecificDownloadDir = filepath.Join(ctx.GetGlobalWorkDir(), defaultContainerdDownloadSubDir)
		logger.Debugf("DownloadDir not specified, using default: %s (derived from global work dir: %s)", hostSpecificDownloadDir, ctx.GetGlobalWorkDir())
    }
    // Temp extraction dir on the target host
    hostSpecificExtractDir := filepath.Join(ctx.GetGlobalWorkDir(), defaultContainerdExtractSubDir, t.Version)
	logger.Debugf("ExtractionDir will be: %s (derived from global work dir: %s)", hostSpecificExtractDir, ctx.GetGlobalWorkDir())


	// --- Phase 1: Download Containerd ---
	downloadStep := component_downloads.NewDownloadContainerdStep(
		t.Version, t.Arch, t.Zone, hostSpecificDownloadDir, t.Checksum,
		cacheKeyContainerdDownloadedPath, // OutputFilePathKey
		"",                               // OutputFileNameKey
		"",                               // OutputComponentTypeKey
		"",                               // OutputVersionKey
		"",                               // OutputArchKey
		"",                               // OutputChecksumKey
		"",                               // OutputURLKey
	)
	downloadPhase := plan.Phase{
		Name:    "Download Containerd Archive",
		Actions: []plan.Action{{Name: "Download containerd archive", Step: downloadStep, Hosts: targetHosts}},
	}
	execPlan.Phases = append(execPlan.Phases, downloadPhase)

	// --- Phase 2: Extract Containerd ---
	extractStep := common.NewExtractArchiveStep(
		cacheKeyContainerdDownloadedPath,
		hostSpecificExtractDir,
		cacheKeyContainerdExtractedPath,
		"",
		false,
		true,
	)
	extractPhase := plan.Phase{
		Name:    "Extract Containerd Archive",
		Actions: []plan.Action{{Name: "Extract containerd archive", Step: extractStep, Hosts: targetHosts}},
	}
	execPlan.Phases = append(execPlan.Phases, extractPhase)

	// --- Phase 3: Install Containerd Binaries ---
	installStep := stepContainerd.NewInstallContainerdStep(
		cacheKeyContainerdExtractedPath,
		nil,
		"",
		"",
		"",
	)
	installPhase := plan.Phase{
		Name:    "Install Containerd Binaries",
		Actions: []plan.Action{{Name: "Install containerd binaries and service file", Step: installStep, Hosts: targetHosts}},
	}
	execPlan.Phases = append(execPlan.Phases, installPhase)

	// --- Phase 4: Configure Containerd ---
	configureStep := stepContainerd.NewConfigureContainerdStep(
		t.RegistryMirrors,
		t.InsecureRegistries,
		t.ContainerdConfigPath,
		t.ExtraTomlContent,
		t.UseSystemdCgroup,
		"",
	)
	configurePhase := plan.Phase{
		Name:    "Configure Containerd",
		Actions: []plan.Action{{Name: "Write containerd config.toml", Step: configureStep, Hosts: targetHosts}},
	}
	execPlan.Phases = append(execPlan.Phases, configurePhase)

	// --- Phase 5: Enable and Start Service ---
	daemonReloadStep := stepContainerd.NewManageContainerdServiceStep(stepContainerd.ServiceActionDaemonReload, "")
    enableServiceStep := stepContainerd.NewManageContainerdServiceStep(stepContainerd.ServiceActionEnable, "")
	startServiceStep := stepContainerd.NewManageContainerdServiceStep(stepContainerd.ServiceActionStart, "")

	manageServicePhase := plan.Phase{
		Name: "Enable and Start Containerd Service",
		Actions: []plan.Action{
		    {Name: "Systemd daemon-reload for containerd", Step: daemonReloadStep, Hosts: targetHosts}, // Name made more specific
			{Name: "Enable containerd service", Step: enableServiceStep, Hosts: targetHosts},
			{Name: "Start containerd service", Step: startServiceStep, Hosts: targetHosts},
		},
	}
	execPlan.Phases = append(execPlan.Phases, manageServicePhase)

	return execPlan, nil
}


// Ensure InstallContainerdTask implements the task.Interface.
var _ task.Interface = (*InstallContainerdTask)(nil)
