package resource

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common" // For ControlNodeHostName
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step" // For step.Step interface
	"github.com/mensylisir/kubexm/pkg/step/common" // Using existing common package for steps
	"github.com/mensylisir/kubexm/pkg/task"      // For task.ExecutionFragment
)

// RemoteBinaryHandle manages a binary that needs to be extracted from a downloaded archive.
// It plans steps to download the archive, extract it, and provide the path to a specific binary within.
type RemoteBinaryHandle struct {
	// ComponentName of the component (e.g., "etcd", "containerd") - used for directory structuring.
	ComponentName string
	// Version of the binary (e.g., "v3.5.4").
	Version string
	// Arch (e.g., "amd64", "arm64"). Must be non-empty.
	Arch string
	// DownloadURL is the direct URL to the archive file.
	DownloadURL string
	// ArchiveFilename is the name of the file as it will be saved locally (e.g., "etcd-v3.5.4-linux-amd64.tar.gz").
	ArchiveFilename string
	// BinaryPathInArchive is the relative path of the target binary within the extracted archive.
	// For example, "etcd-v3.5.4-linux-amd64/etcd" or "bin/containerd".
	BinaryPathInArchive string
	// ExpectedChecksum is the SHA256 checksum of the downloaded archive file for verification. Optional.
	ExpectedChecksum string
	// ChecksumAlgorithm is the algorithm for the checksum (e.g., "sha256"). Defaults to "sha256" if ExpectedChecksum is set.
	ChecksumAlgorithm string
	// OS a.k.a. GOOS, e.g. "linux", "darwin". Used if archive name needs it. Often "linux".
	OS string
}

// NewRemoteBinaryHandle creates a new RemoteBinaryHandle.
// It requires essential parameters and attempts to determine Arch if not provided.
func NewRemoteBinaryHandle(
	ctx runtime.TaskContext, // TaskContext needed to resolve Arch if not provided
	componentName, version, arch, osName,
	downloadURLTemplate, archiveFilenameTemplate, binaryPathInArchive,
	checksum string, checksumAlgo string,
) (Handle, error) {
	if componentName == "" || version == "" || downloadURLTemplate == "" || archiveFilenameTemplate == "" || binaryPathInArchive == "" {
		return nil, fmt.Errorf(
			"missing required parameters for RemoteBinaryHandle (componentName, version, downloadURLTemplate, archiveFilenameTemplate, binaryPathInArchive must be set)",
		)
	}

	finalArch := arch
	if finalArch == "" {
		// Attempt to derive Arch from the control node's facts
		controlNode, err := ctx.GetControlNode()
		if err != nil {
			return nil, fmt.Errorf("cannot determine arch: failed to get control node: %w", err)
		}
		facts, err := ctx.GetHostFacts(controlNode)
		if err != nil {
			return nil, fmt.Errorf("cannot determine arch: failed to get control node facts: %w", err)
		}
		if facts.OS == nil || facts.OS.Arch == "" {
			return nil, fmt.Errorf("cannot determine arch: control node OS.Arch is empty")
		}
		finalArch = facts.OS.Arch
		if finalArch == "x86_64" {
			finalArch = "amd64"
		} else if finalArch == "aarch64" {
			finalArch = "arm64"
		}
		ctx.GetLogger().Info("Architecture for RemoteBinaryHandle auto-derived.", "component", componentName, "arch", finalArch)
	}

	finalOS := osName
	if finalOS == "" {
		finalOS = "linux" // Default to Linux if OS is not specified
		ctx.GetLogger().Info("OS for RemoteBinaryHandle defaulted to linux.", "component", componentName)
	}

	if checksum != "" && checksumAlgo == "" {
		checksumAlgo = "sha256" // Default to sha256 if checksum is provided
	}

	// Prepare template data
	templateData := struct {
		Version  string
		Arch     string
		OS       string
		FileName string // Filename itself can be a template output
	}{
		Version: version,
		Arch:    finalArch,
		OS:      finalOS,
	}

	// Process ArchiveFileName template
	// First, if ArchiveFileName is a template for itself (e.g. contains {{.OS}})
	parsedArchiveFilename, err := common.RenderTemplate(archiveFilenameTemplate, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render archive filename template '%s': %w", archiveFilenameTemplate, err)
	}
	templateData.FileName = parsedArchiveFilename // Make the rendered filename available for URL template

	// Process DownloadURL template
	finalDownloadURL, err := common.RenderTemplate(downloadURLTemplate, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render download URL template '%s': %w", downloadURLTemplate, err)
	}

	return &RemoteBinaryHandle{
		ComponentName:       componentName,
		Version:             version,
		Arch:                finalArch, // Use the determined Arch
		OS:                  finalOS,   // Use the determined OS
		DownloadURL:         finalDownloadURL,
		ArchiveFilename:     parsedArchiveFilename, // Use the rendered archive filename
		BinaryPathInArchive: binaryPathInArchive,
		ExpectedChecksum:    checksum,
		ChecksumAlgorithm:   checksumAlgo,
	}, nil
}

// ID generates a unique identifier for this resource instance.
func (h *RemoteBinaryHandle) ID() string {
	return fmt.Sprintf("%s-binary-%s-%s-%s-%s",
		strings.ToLower(h.ComponentName),
		h.Version,
		h.OS,
		h.Arch,
		strings.ReplaceAll(h.ArchiveFilename, "/", "_"))
}

// Path returns the expected local path of the target binary after download and extraction.
// Path: ${GlobalWorkDir}/${cluster_name}/${ComponentName}/${Version}/${Arch}/<filename_part_of_BinaryPathInArchive>
func (h *RemoteBinaryHandle) Path(ctx runtime.TaskContext) (string, error) {
	if ctx.GetClusterConfig() == nil { // Should be caught by TaskContext usage if it provides GetClusterConfig
		return "", fmt.Errorf("cluster config is nil in TaskContext, cannot determine path for %s", h.ID())
	}
	if h.Arch == "" {
		return "", fmt.Errorf("architecture is not set for RemoteBinaryHandle %s", h.ID())
	}

	// Base directory for this component's versioned and arched artifacts
	// e.g. $(pwd)/.kubexm/mycluster/etcd/v3.5.4/amd64/
	baseArtifactDir := ctx.GetFileDownloadPath(h.ComponentName, h.Version, h.Arch, "")

	finalBinaryName := filepath.Base(h.BinaryPathInArchive)
	return filepath.Join(baseArtifactDir, finalBinaryName), nil
}

// EnsurePlan generates an ExecutionFragment to download and extract the archive, making the binary available.
// Steps are planned to run on the local control node.
func (h *RemoteBinaryHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_id", h.ID(), "component", h.ComponentName)
	logger.Info("Planning resource assurance for remote binary...")

	finalBinaryPath, err := h.Path(ctx)
	if err != nil {
		logger.Error(err, "Failed to determine final binary path")
		return nil, fmt.Errorf("failed to determine final binary path for %s: %w", h.ID(), err)
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		logger.Error(err, "Failed to get control node from TaskContext")
		return nil, fmt.Errorf("failed to get control node for resource %s: %w", h.ID(), err)
	}

	// Path for the downloaded archive:
	// ${GlobalWorkDir}/${cluster_name}/${ComponentName}/${Version}/${Arch}/${ArchiveFilename}
	localArchivePath := ctx.GetFileDownloadPath(h.ComponentName, h.Version, h.Arch, h.ArchiveFilename)
	localArchiveDir := filepath.Dir(localArchivePath)

	// Directory for extraction:
	// ${GlobalWorkDir}/${cluster_name}/${ComponentName}/${Version}/${Arch}/extracted_<ArchiveFilename_without_ext>/
	archiveNameWithoutExt := strings.TrimSuffix(h.ArchiveFilename, filepath.Ext(h.ArchiveFilename))
	extractionDir := filepath.Join(localArchiveDir, "extracted_"+archiveNameWithoutExt)

	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID

	// --- Pre-check if final binary already exists and is valid ---
	runner := ctx.GetRunner()
	localConn, err := ctx.GetConnectorForHost(controlNode)
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for control node during pre-check: %w", err)
	}
	if exists, _ := runner.Exists(ctx.GoContext(), localConn, finalBinaryPath); exists {
		// TODO: Add checksum for the final binary itself if provided by the handle.
		logger.Info("Final binary already exists. Assuming valid and skipping resource acquisition.", "path", finalBinaryPath)
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}

	// 1. Download Step
	downloadNodeID := plan.NodeID(fmt.Sprintf("download-%s", h.ID()))
	downloadStep := common.NewDownloadFileStep(
		h.DownloadURL,
		localArchiveDir, // Directory to download into
		h.ArchiveFilename,
		h.ExpectedChecksum, // Checksum for the archive
		h.ChecksumAlgorithm,
		"0644", // Permissions for the downloaded archive
	)
	nodes[downloadNodeID] = &plan.ExecutionNode{
		Name:     fmt.Sprintf("Download %s archive (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
		Step:     downloadStep,
		Hosts:    []connector.Host{controlNode},
		StepName: "DownloadFile", // TODO: Replace with step.Meta().Name
	}
	entryNodes = append(entryNodes, downloadNodeID)

	// 2. Extract Step
	extractNodeID := plan.NodeID(fmt.Sprintf("extract-%s", h.ID()))
	extractStep := common.NewExtractArchiveStep(
		localArchivePath, // Source is the downloaded archive
		extractionDir,    // Destination is the dedicated extraction directory
		"",               // Specific file to extract (empty for all)
	)
	nodes[extractNodeID] = &plan.ExecutionNode{
		Name:         fmt.Sprintf("Extract %s archive (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
		Step:         extractStep,
		Hosts:        []connector.Host{controlNode},
		Dependencies: []plan.NodeID{downloadNodeID},
		StepName:     "ExtractArchive", // TODO: Replace with step.Meta().Name
	}

	// 3. Finalize Binary Step (Copy from extraction path to final path and ensure executable)
	sourceBinaryInExtraction := filepath.Join(extractionDir, h.BinaryPathInArchive)
	finalizeNodeID := plan.NodeID(fmt.Sprintf("finalize-%s", h.ID()))

	cmdToFinalize := fmt.Sprintf("mkdir -p %s && cp -f %s %s && chmod +x %s",
		filepath.Dir(finalBinaryPath),
		sourceBinaryInExtraction,
		finalBinaryPath,
		finalBinaryPath,
	)
	finalizeStep := common.NewCommandStep(cmdToFinalize, "", "", false, "", 0, nil)
	nodes[finalizeNodeID] = &plan.ExecutionNode{
		Name:         fmt.Sprintf("Finalize %s binary (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
		Step:         finalizeStep,
		Hosts:        []connector.Host{controlNode},
		Dependencies: []plan.NodeID{extractNodeID},
		StepName:     "Command", // TODO: Replace with step.Meta().Name
	}
	exitNodes = append(exitNodes, finalizeNodeID)

	logger.Info("Remote binary resource assurance plan created.", "final_binary_path", finalBinaryPath)
	return &task.ExecutionFragment{
		Nodes:      nodes,
		EntryNodes: entryNodes,
		ExitNodes:  exitNodes,
	}, nil
}

// Ensure RemoteBinaryHandle implements the Handle interface.
var _ Handle = (*RemoteBinaryHandle)(nil)
