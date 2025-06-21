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
	task.BaseTask
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
	runOnRoles []string, // This will be passed to BaseTask
) task.Task { // Returns task.Task
	return &InstallContainerdTask{
		BaseTask: task.NewBaseTask(
			"InstallAndConfigureContainerd", // TaskName
			fmt.Sprintf("Installs and configures containerd version %s", version), // TaskDesc
			runOnRoles, // RunOnRoles
			nil,        // HostFilter - can be added if needed
			false,      // IgnoreError
		),
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

// IsRequired can be overridden if default BaseTask logic is not sufficient.
// For this task, the default BaseTask.IsRequired (true) combined with role-based
// host filtering in Plan is likely sufficient. Or, provide a custom one:
func (t *InstallContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// If RunOnRoles is empty, this task might be intended for all nodes
	// or might be skipped. The Plan logic will handle empty targetHosts.
	// BaseTask.IsRequired returns true by default.
	// If specific logic is needed to disable this task entirely based on config:
	// clusterConfig := ctx.GetClusterConfig()
	// if clusterConfig.Spec.ContainerRuntime.Type != "containerd" { return false, nil }
	return t.BaseTask.IsRequired(ctx) // Or custom logic
}

// Plan generates the execution fragment to install and configure containerd.
func (t *InstallContainerdTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID

	// 1. Determine target hosts using RunOnRoles from BaseTask
	targetHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}

	if len(targetHosts) == 0 {
		logger.Info("No target hosts found for InstallContainerdTask based on roles, returning empty fragment.")
		return task.NewEmptyFragment(), nil
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
	nodes[nodeIDDownload] = &plan.ExecutionNode{Name: "Download Containerd Archive", Step: downloadStep, Hosts: targetHosts, StepName: downloadStep.Meta().Name}
	nodes[nodeIDExtract] = &plan.ExecutionNode{Name: "Extract Containerd Archive", Step: extractStep, Hosts: targetHosts, Dependencies: []plan.NodeID{nodeIDDownload}, StepName: extractStep.Meta().Name}
	nodes[nodeIDInstall] = &plan.ExecutionNode{Name: "Install Containerd Binaries", Step: installBinariesStep, Hosts: targetHosts, Dependencies: []plan.NodeID{nodeIDExtract}, StepName: installBinariesStep.Meta().Name}
	nodes[nodeIDConfigure] = &plan.ExecutionNode{Name: "Configure Containerd Service", Step: configureServiceStep, Hosts: targetHosts, Dependencies: []plan.NodeID{nodeIDInstall}, StepName: configureServiceStep.Meta().Name}
	nodes[nodeIDDaemonReload] = &plan.ExecutionNode{Name: "Daemon Reload for Containerd", Step: daemonReloadServiceStep, Hosts: targetHosts, Dependencies: []plan.NodeID{nodeIDConfigure}, StepName: daemonReloadServiceStep.Meta().Name}
	nodes[nodeIDEnable] = &plan.ExecutionNode{Name: "Enable Containerd Service", Step: enableServiceStep, Hosts: targetHosts, Dependencies: []plan.NodeID{nodeIDDaemonReload}, StepName: enableServiceStep.Meta().Name}
	nodes[nodeIDStart] = &plan.ExecutionNode{Name: "Start Containerd Service", Step: startServiceStep, Hosts: targetHosts, Dependencies: []plan.NodeID{nodeIDEnable}, StepName: startServiceStep.Meta().Name}

	entryNodes = append(entryNodes, nodeIDDownload)
	exitNodes = append(exitNodes, nodeIDStart)

	return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
}

// Ensure InstallContainerdTask implements the task.Task interface.
var _ task.Task = (*InstallContainerdTask)(nil) // Changed from task.Interface
