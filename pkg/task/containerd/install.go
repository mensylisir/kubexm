package containerd

import (
	"fmt"
	"path/filepath" // For joining paths for temporary extraction

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task" // Import task package for task.Task and task.ExecutionFragment

	// Import step packages
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/component_downloads"
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
)

const (
	// Cache key constants for this task
	cacheKeyContainerdDownloadedPath = "InstallContainerdTask.ContainerdDownloadedPath"
	cacheKeyContainerdExtractedPath  = "InstallContainerdTask.ContainerdExtractedPath"
	// Default download directory on target host, relative to global work dir
	defaultContainerdDownloadSubDir = "downloads/containerd"
	defaultContainerdExtractSubDir  = "extract/containerd"
)

// InstallContainerdTask installs and configures containerd.
type InstallContainerdTask struct {
	// BaseTask removed, methods to be implemented directly or via a compatible base
	taskName             string
	taskDesc             string
	runOnRoles           []string
	Version              string
	Arch                 string
	Zone                 string
	DownloadDir          string
	Checksum             string
	RegistryMirrors      map[string]string
	InsecureRegistries   []string
	UseSystemdCgroup     bool
	ExtraTomlContent     string
	ContainerdConfigPath string
}

// NewInstallContainerdTask creates a new task for installing and configuring containerd.
func NewInstallContainerdTask(
	version string, arch string, zone string, downloadDir string, checksum string,
	registryMirrors map[string]string, insecureRegistries []string,
	useSystemdCgroup bool, extraToml string, configPath string,
	runOnRoles []string,
) task.Task { // Returns task.Task
	return &InstallContainerdTask{
		taskName:             "InstallAndConfigureContainerd",
		taskDesc:             fmt.Sprintf("Installs and configures containerd version %s", version),
		runOnRoles:           runOnRoles,
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

// Name returns the name of the task.
func (t *InstallContainerdTask) Name() string {
	return t.taskName
}

// IsRequired determines if the task needs to run.
func (t *InstallContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if len(t.runOnRoles) == 0 {
		// If no roles specified, task is usually skipped unless it's a control-plane-only task.
		// Or, if it's meant to run on all hosts, logic here would confirm that.
		// For now, consistent with BaseTask: if no roles, it's not required for specific role-based execution.
		return false, nil // Or true if it's a global task not tied to specific roles.
	}
	// Check if any host matches the roles for this task.
	for _, role := range t.runOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			return false, fmt.Errorf("failed to get hosts for role '%s' in task %s: %w", role, t.Name(), err)
		}
		if len(hosts) > 0 {
			return true, nil // Required if at least one host matches one of the roles.
		}
	}
	return false, nil // Not required if no hosts match any of the specified roles.
}

// Plan generates the execution fragment to install and configure containerd.
func (t *InstallContainerdTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID

	// 1. Determine target hosts
	var targetHosts []connector.Host
	if len(t.runOnRoles) == 0 {
		logger.Info("No specific roles defined for InstallContainerdTask.")
		// Depending on task nature, could target all hosts or control-plane nodes by default.
		// For now, if no roles, this task generates an empty fragment.
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}
	for _, role := range t.runOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			return nil, fmt.Errorf("failed to get hosts for role '%s' in task %s: %w", role, t.Name(), err)
		}
		targetHosts = append(targetHosts, hosts...)
	}

	if len(targetHosts) == 0 {
		logger.Info("No target hosts found for InstallContainerdTask after role filtering.")
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}
	// Deduplicate hosts
	uniqueHosts := make(map[string]connector.Host)
	for _, h := range targetHosts {
		uniqueHosts[h.GetName()] = h
	}
	targetHosts = []connector.Host{} // Clear and repopulate
	for _, h := range uniqueHosts {
		targetHosts = append(targetHosts, h)
	}

	hostNamesForLog := []string{}
	for _, h := range targetHosts {
		hostNamesForLog = append(hostNamesForLog, h.GetName())
	}
	logger.Infof("Planning containerd installation for %d hosts: %v", len(targetHosts), hostNamesForLog)

	hostSpecificDownloadDir := t.DownloadDir
	if hostSpecificDownloadDir == "" {
		hostSpecificDownloadDir = filepath.Join(ctx.GetGlobalWorkDir(), defaultContainerdDownloadSubDir)
		logger.Debugf("DownloadDir not specified, using default: %s", hostSpecificDownloadDir)
	}
	hostSpecificExtractDir := filepath.Join(ctx.GetGlobalWorkDir(), defaultContainerdExtractSubDir, t.Version)
	logger.Debugf("ExtractionDir will be: %s", hostSpecificExtractDir)

	// Node IDs
	nodeIDDownload := plan.NodeID(fmt.Sprintf("download-containerd-%s", t.Version))
	nodeIDExtract := plan.NodeID(fmt.Sprintf("extract-containerd-%s", t.Version))
	nodeIDInstall := plan.NodeID(fmt.Sprintf("install-containerd-binaries-%s", t.Version))
	nodeIDConfigure := plan.NodeID(fmt.Sprintf("configure-containerd-%s", t.Version))
	nodeIDDaemonReload := plan.NodeID(fmt.Sprintf("daemon-reload-containerd-%s", t.Version))
	nodeIDEnable := plan.NodeID(fmt.Sprintf("enable-containerd-service-%s", t.Version))
	nodeIDStart := plan.NodeID(fmt.Sprintf("start-containerd-service-%s", t.Version))

	// Create Steps
	downloadStep := component_downloads.NewDownloadContainerdStep(
		"DownloadContainerdArchive", t.Version, t.Arch, t.Zone, hostSpecificDownloadDir, t.Checksum, true, /* TODO: sudo for download dir? */
		cacheKeyContainerdDownloadedPath, "", "", "", "", "", "",
	)
	extractStep := common.NewExtractArchiveStep(
		"ExtractContainerdArchive", cacheKeyContainerdDownloadedPath, hostSpecificExtractDir, cacheKeyContainerdExtractedPath,
		"", true, /* TODO: sudo for extract? */
		false, true,
	)
	installBinariesStep := stepContainerd.NewInstallContainerdStep( // Constructor updated in step refactor
		"InstallContainerdBinaries", cacheKeyContainerdExtractedPath, nil, "", "", true, /* sudo for install */
	)
	configureServiceStep := stepContainerd.NewConfigureContainerdStep( // Constructor updated in step refactor
		"ConfigureContainerdService", t.RegistryMirrors, t.InsecureRegistries, t.ContainerdConfigPath,
		t.ExtraTomlContent, t.UseSystemdCgroup, true, /* sudo for config */
	)
	daemonReloadServiceStep := stepContainerd.NewManageContainerdServiceStep("DaemonReloadForContainerd", stepContainerd.ServiceActionDaemonReload, true)
	enableServiceStep := stepContainerd.NewManageContainerdServiceStep("EnableContainerdService", stepContainerd.ServiceActionEnable, true)
	startServiceStep := stepContainerd.NewManageContainerdServiceStep("StartContainerdService", stepContainerd.ServiceActionStart, true)

	// Create Nodes
	nodes[nodeIDDownload] = &plan.ExecutionNode{Step: downloadStep, Hosts: targetHosts, Name: "Download Containerd Archive"}
	nodes[nodeIDExtract] = &plan.ExecutionNode{Step: extractStep, Hosts: targetHosts, Name: "Extract Containerd Archive", Dependencies: []plan.NodeID{nodeIDDownload}}
	nodes[nodeIDInstall] = &plan.ExecutionNode{Step: installBinariesStep, Hosts: targetHosts, Name: "Install Containerd Binaries", Dependencies: []plan.NodeID{nodeIDExtract}}
	nodes[nodeIDConfigure] = &plan.ExecutionNode{Step: configureServiceStep, Hosts: targetHosts, Name: "Configure Containerd Service", Dependencies: []plan.NodeID{nodeIDInstall}}
	nodes[nodeIDDaemonReload] = &plan.ExecutionNode{Step: daemonReloadServiceStep, Hosts: targetHosts, Name: "Daemon Reload for Containerd", Dependencies: []plan.NodeID{nodeIDConfigure}}
	nodes[nodeIDEnable] = &plan.ExecutionNode{Step: enableServiceStep, Hosts: targetHosts, Name: "Enable Containerd Service", Dependencies: []plan.NodeID{nodeIDDaemonReload}}
	nodes[nodeIDStart] = &plan.ExecutionNode{Step: startServiceStep, Hosts: targetHosts, Name: "Start Containerd Service", Dependencies: []plan.NodeID{nodeIDEnable}}

	entryNodes = append(entryNodes, nodeIDDownload)
	exitNodes = append(exitNodes, nodeIDStart)

	return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
}

// Ensure InstallContainerdTask implements the task.Task interface.
var _ task.Task = (*InstallContainerdTask)(nil) // Changed from task.Interface
