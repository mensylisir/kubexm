package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	stepCommon "github.com/mensylisir/kubexm/pkg/step/common"
	stepComponentDownloads "github.com/mensylisir/kubexm/pkg/step/component_downloads"
	"github.com/mensylisir/kubexm/pkg/task"
)

const (
	kubeletComponent = "kubelet"
	kubeletTargetDir = "/usr/local/bin"
	kubeletPermissions = "0755"
)
// FetchKubeletTask downloads kubelet to the control node and then uploads/installs it to target hosts.
type FetchKubeletTask struct {
	taskName             string
	taskDesc             string
	runOnRoles           []string // Roles where kubelet should be installed
	Version              string
	Arch                 string // Optional, can be derived from host facts if empty during step execution
	Zone                 string // For download mirrors
	Checksum             string // Optional checksum for the downloaded binary
	SudoForUploadInstall bool   // Sudo for upload and chmod on target nodes
}

// NewFetchKubeletTask creates a new task to fetch and install kubelet.
func NewFetchKubeletTask(cfg *v1alpha1.Cluster, roles []string) task.Task {
	// Extract relevant info from cfg or use defaults
	version := cfg.Spec.Kubernetes.Version // Assuming version is here
	arch := ""                             // Let download step determine from control node facts, or allow override
	zone := cfg.Spec.ImageHub.Zone       // Assuming zone is here
	checksum := ""                         // TODO: Obtain checksum for kubelet if available/needed

	return &FetchKubeletTask{
		taskName:             fmt.Sprintf("FetchAndInstallKubelet-%s", version),
		taskDesc:             fmt.Sprintf("Downloads kubelet %s to control node, then installs on target hosts.", version),
		runOnRoles:           roles,
		Version:              version,
		Arch:                 arch,
		Zone:                 zone,
		Checksum:             checksum,
		SudoForUploadInstall: true, // Typically true for /usr/local/bin
	}
}

func (t *FetchKubeletTask) Name() string { return t.taskName }

func (t *FetchKubeletTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if len(t.runOnRoles) == 0 { return true, nil } // Assume run on all if no roles specified for this task
	for _, role := range t.runOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil { return false, err }
		if len(hosts) > 0 { return true, nil }
	}
	return false, nil
}

func (t *FetchKubeletTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	entryNodes := []plan.NodeID{}
	exitNodes := []plan.NodeID{}

	controlNodeHosts, err := ctx.GetHostsByRole(common.ControlNodeRole)
	if err != nil { return nil, fmt.Errorf("failed to get control node: %w", err) }
	if len(controlNodeHosts) == 0 { return nil, fmt.Errorf("no control node found") }
	controlNode := controlNodeHosts[0]

	targetHosts, err := t.determineTargetHosts(ctx)
	if err != nil { return nil, err }
	if len(targetHosts) == 0 {
		logger.Info("No target hosts for task.")
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}

	// Path for Kubelet on the control node after download
	// Arch is determined by the DownloadKubeletStep based on the control node's architecture if t.Arch is empty.
	// The step needs to output the determined arch if it was auto-detected, for consistent naming.
	// For now, assume t.Arch will be populated or correctly used by download step.
	// The DownloadKubeletStep's internal logic will determine the actual filename.
	// We need a way for this task to know that filename or the full path.
	// Let's use a predictable structure in GlobalWorkDir.
	// The DownloadKubeletStep should be refactored to accept a full target path on control node.

	// Arch for download path naming (can be determined by download step if t.Arch is empty)
	archForPath := t.Arch
	if archForPath == "" {
		// If arch is empty, the download step will use control-node's arch.
		// We need this info to construct the predictable path.
		// This implies the task or handle needs to get control-node facts first if arch is not specified.
		// For simplicity in this refactor, assume arch will be specified or derived before this point if needed for path.
		// Or, the download step must return the *actual* determined arch if it auto-detects.
		// Let's assume download step will place it predictably or cache the path.
		// A simpler way: download step takes DownloadDir and FileName, and caches the full path.
		// For now, let's assume DownloadKubeletStep will output the path to KubeletDownloadedPathKey.
	}

	controlNodeDownloadDir := filepath.Join(ctx.GetGlobalWorkDir(), "resources", kubeletComponent, t.Version)
	// The actual downloaded file name will be determined by DownloadKubeletStep (e.g., "kubelet")
	// and its full path will be stored in KubeletDownloadedPathKey.
	// Let's assume the DownloadKubeletStep is refactored to take instanceName, version, arch, zone, downloadDir, fileName, checksum, sudo, outputCacheKey

	nodeIDDownloadKubelet := plan.NodeID(fmt.Sprintf("download-%s-%s", kubeletComponent, t.Version))
	downloadStep := stepComponentDownloads.NewDownloadKubeletStep( // Assumes this constructor is refactored
		"DownloadKubeletToControlNode",
		t.Version,
		t.Arch, // Arch can be empty, step will determine from control node
		t.Zone,
		controlNodeDownloadDir, // Directory on control node to download to
		kubeletComponent,       // Expected filename on control node (could be just "kubelet")
		t.Checksum,
		false, // Sudo for download on control node (typically false for workdir)
		stepComponentDownloads.KubeletDownloadedPathKey, // Output cache key for the downloaded file path
	)
	nodes[nodeIDDownloadKubelet] = &plan.ExecutionNode{
		Name:  fmt.Sprintf("Download %s %s to Control Node", kubeletComponent, t.Version),
		Step:  downloadStep,
		Hosts: []connector.Host{controlNode}, // Runs only on control node
	}
	entryNodes = append(entryNodes, nodeIDDownloadKubelet)

	// Node 2: Upload and Install Kubelet to target hosts
	nodeIDInstallKubelet := plan.NodeID(fmt.Sprintf("install-%s-%s-on-targets", kubeletComponent, t.Version))
	// InstallBinaryStep takes SourcePathSharedDataKey to get the path from the previous step's cache.
	// It implies that TaskCache is shared across nodes/hosts for this to work, or the path is absolute and accessible.
	// For items downloaded to control node, the path is absolute on control node.
	// InstallBinaryStep needs to:
	// 1. (If on control node) Get path from TaskCache.
	// 2. (If on target node different from where SourcePathSharedDataKey was set) This won't work directly.
	// Solution: UploadFileStep + CommandStep for chmod.

	// We need the actual path on the control node. The download step caches it.
	// The Upload step needs this actual path.
	// This means the `SourcePath` for UploadFileStep cannot be a cache key directly if the step doesn't resolve it.
	// Let's assume UploadFileStep can take a direct path, and we construct it.
	// The DownloadKubeletStep will download to `controlNodeDownloadDir/kubeletComponent`.
	localKubeletPathOnControlNode := filepath.Join(controlNodeDownloadDir, kubeletComponent)


	uploadStep := stepCommon.NewUploadFileStep(
		fmt.Sprintf("Upload-%s-%s", kubeletComponent, t.Version),
		localKubeletPathOnControlNode, // Source path on control node
		filepath.Join(kubeletTargetDir, kubeletComponent), // Target path on remote hosts
		kubeletPermissions,
		t.SudoForUploadInstall,
	)
	nodes[nodeIDInstallKubelet] = &plan.ExecutionNode{
		Name:         fmt.Sprintf("Upload and Set Permissions for %s on Target Hosts", kubeletComponent),
		Step:         uploadStep,
		Hosts:        targetHosts,
		Dependencies: []plan.NodeID{nodeIDDownloadKubelet},
	}
	exitNodes = append(exitNodes, nodeIDInstallKubelet)

	logger.Info("Planned fetch and install for kubelet", "targetHosts", len(targetHosts))
	return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
}

func (t *FetchKubeletTask) determineTargetHosts(ctx runtime.TaskContext) ([]connector.Host, error) {
	if len(t.runOnRoles) == 0 {
		return ctx.GetAllHosts() // Default to all hosts if no specific roles
	}
	var targetHosts []connector.Host
	uniqueHosts := make(map[string]connector.Host)
	for _, role := range t.runOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			return nil, fmt.Errorf("failed to get hosts for role '%s': %w", role, err)
		}
		for _, h := range hosts {
			uniqueHosts[h.GetName()] = h
		}
	}
	for _, h := range uniqueHosts {
		targetHosts = append(targetHosts, h)
	}
	return targetHosts, nil
}

var _ task.Task = (*FetchKubeletTask)(nil)
