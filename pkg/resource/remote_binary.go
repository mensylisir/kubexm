package resource

import (
	"fmt"
	"path/filepath"
	"strings"

	// "github.com/mensylisir/kubexm/pkg/common" // No longer needed for DefaultEtcdDir etc. directly here
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/command"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // Aliased to avoid conflict if common was a var
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/util" // For util.GetBinaryInfo and util.BinaryInfo
)

// RemoteBinaryHandle manages a binary that needs to be acquired, potentially from an archive.
// It plans steps to download the source (archive or direct binary), extract it if necessary,
// and provide the path to a specific binary.
type RemoteBinaryHandle struct {
	// ComponentName of the component (e.g., "etcd", "containerd"). Used for logging and potentially deriving BinaryInfo.
	ComponentName string
	// Version of the binary (e.g., "v3.5.4").
	Version string
	// Arch (e.g., "amd64", "arm64"). If empty, will try to determine from control node.
	Arch string
	// OS (e.g., "linux"). If empty, will default based on component or to "linux".
	OS string // Store resolved OS for ID generation and consistency
	// BinaryNameInArchive is the specific name or relative path of the target binary
	// if the downloaded item is an archive (e.g., "etcd", "bin/containerd").
	// If the downloaded item is the binary itself, this should match its filename or be empty.
	BinaryNameInArchive string
	// ExpectedChecksum is the SHA256 checksum of the downloaded file (archive or direct binary) for verification. Optional.
	ExpectedChecksum string
	// ChecksumAlgorithm is the algorithm for the checksum. Defaults to "sha256" if ExpectedChecksum is set.
	ChecksumAlgorithm string

	// Internal field to store resolved binary information
	binaryInfo *util.BinaryInfo
}

// NewRemoteBinaryHandle creates a new RemoteBinaryHandle.
// It calls util.GetBinaryInfo to resolve download URLs and filenames.
// binaryNameInArchive is the name of the *specific target binary* within the archive,
// or the name of the binary itself if it's not an archive.
func NewRemoteBinaryHandle(
	ctx task.TaskContext, // Needed for GetBinaryInfo (workDir, clusterName) and logger
	componentName, version, arch, osName, binaryNameInArchive string,
	expectedChecksum, checksumAlgo string,
) (Handle, error) {
	if componentName == "" || version == "" {
		return nil, fmt.Errorf("componentName and version are required for RemoteBinaryHandle")
	}
	logger := ctx.GetLogger().With("component", componentName, "version", version)

	// Get workDir and clusterName from context for GetBinaryInfo
	// util.GetBinaryInfo expects workDir to be the directory *containing* '.kubexm'
	// ctx.GetGlobalWorkDir() is '$(pwd)/.kubexm/${cluster_name}'
	// So, we need to get the parent of the parent of GlobalWorkDir.
	globalWorkDir := ctx.GetGlobalWorkDir()
	if globalWorkDir == "" {
		return nil, fmt.Errorf("GlobalWorkDir is empty in TaskContext, cannot create RemoteBinaryHandle for %s", componentName)
	}
	baseWorkDirForBinaryInfo := filepath.Dir(filepath.Dir(globalWorkDir)) // This should be $(pwd)

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		return nil, fmt.Errorf("cluster config is nil in TaskContext, cannot create RemoteBinaryHandle for %s", componentName)
	}
	clusterName := clusterCfg.Name
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is empty in TaskContext, cannot create RemoteBinaryHandle for %s", componentName)
	}

	finalArch := arch
	if finalArch == "" {
		controlNode, err := ctx.GetControlNode()
		if err != nil {
			return nil, fmt.Errorf("cannot auto-determine arch for %s: failed to get control node: %w", componentName, err)
		}
		facts, err := ctx.GetHostFacts(controlNode)
		if err != nil {
			return nil, fmt.Errorf("cannot auto-determine arch for %s: failed to get control node facts: %w", componentName, err)
		}
		if facts.OS == nil || facts.OS.Arch == "" {
			return nil, fmt.Errorf("cannot auto-determine arch for %s: control node OS.Arch is empty", componentName)
		}
		finalArch = facts.OS.Arch
		// Normalize common arch names if needed (e.g., x86_64 -> amd64)
		if finalArch == "x86_64" { finalArch = "amd64" }
		if finalArch == "aarch64" { finalArch = "arm64" }
		logger.Debug("Architecture auto-derived for RemoteBinaryHandle.", "component", componentName, "arch", finalArch)
	}

	finalOS := osName
	// util.GetBinaryInfo will use its own default for OS if osName is empty

	binInfo, err := util.GetBinaryInfo(componentName, version, finalArch, util.GetZone(), baseWorkDirForBinaryInfo, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get binary info for %s@%s (%s): %w", componentName, version, finalArch, err)
	}

	actualBinaryNameInArchive := binaryNameInArchive
	if !binInfo.IsArchive && actualBinaryNameInArchive == "" {
		actualBinaryNameInArchive = binInfo.FileName
	}

	finalChecksumAlgo := checksumAlgo
	if expectedChecksum != "" && finalChecksumAlgo == "" {
		finalChecksumAlgo = "sha256"
	}

	return &RemoteBinaryHandle{
		ComponentName:       componentName,
		Version:             version,
		Arch:                binInfo.Arch, // Use resolved Arch from binInfo
		OS:                  binInfo.OS,   // Use resolved OS from binInfo
		BinaryNameInArchive: actualBinaryNameInArchive,
		ExpectedChecksum:    expectedChecksum,
		ChecksumAlgorithm:   finalChecksumAlgo,
		binaryInfo:          binInfo,
	}, nil
}

// ID generates a unique identifier for this resource instance.
func (h *RemoteBinaryHandle) ID() string {
	if h.binaryInfo == nil {
		// Fallback or error if binaryInfo isn't initialized (should not happen if constructor is used)
		return fmt.Sprintf("%s-binary-%s-%s-%s-nobinfo",
			strings.ToLower(h.ComponentName), h.Version, h.OS, h.Arch)
	}
	idString := fmt.Sprintf("%s-%s-%s-%s-%s",
		strings.ToLower(h.ComponentName),
		h.Version, // Use original version for consistency in ID if it differs from binInfo.Version (e.g. v-prefix)
		h.binaryInfo.OS,
		h.binaryInfo.Arch,
		strings.ReplaceAll(h.binaryInfo.FileName, "/", "_"))
	if h.binaryInfo.IsArchive && h.BinaryNameInArchive != "" {
		idString += fmt.Sprintf("-target-%s", strings.ReplaceAll(h.BinaryNameInArchive, "/", "_"))
	}
	return idString
}

// Path returns the expected local path of the target binary on the control node
// after it has been successfully acquired and prepared.
func (h *RemoteBinaryHandle) Path(ctx task.TaskContext) (string, error) {
	if h.binaryInfo == nil {
		return "", fmt.Errorf("binaryInfo not initialized for RemoteBinaryHandle %s, call NewRemoteBinaryHandle first", h.ComponentName)
	}

	if h.binaryInfo.IsArchive {
		if h.BinaryNameInArchive == "" {
			// Path to the downloaded archive itself
			return h.binaryInfo.FilePath, nil
		}
		// Path to the specific binary *within* the extracted archive contents
		archiveBase := strings.TrimSuffix(h.binaryInfo.FileName, filepath.Ext(h.binaryInfo.FileName))
		if strings.HasSuffix(archiveBase, ".tar") { // Handle .tar.gz by removing .tar as well
			archiveBase = strings.TrimSuffix(archiveBase, ".tar")
		}
		extractionDir := filepath.Join(h.binaryInfo.ComponentDir, "extracted_"+archiveBase)
		return filepath.Join(extractionDir, h.BinaryNameInArchive), nil
	}
	// Not an archive, so path is directly to the downloaded binary file (which is h.binaryInfo.FilePath)
	return h.binaryInfo.FilePath, nil
}

// EnsurePlan generates an ExecutionFragment to download and (if necessary) extract the binary,
// making the target binary available at the path returned by h.Path().
// Steps are planned to run on the local control node.
func (h *RemoteBinaryHandle) EnsurePlan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_id", h.ID(), "component", h.ComponentName)
	logger.Info("Planning resource assurance for remote binary...")

	if h.binaryInfo == nil {
		return nil, fmt.Errorf("binaryInfo not initialized for handle %s, cannot plan", h.ID())
	}

	finalBinaryTargetPath, err := h.Path(ctx) // This is the path to the *target* binary (e.g. extracted etcd, or downloaded runc)
	if err != nil {
		logger.Error(err, "Failed to determine final binary target path")
		return nil, fmt.Errorf("failed to determine final binary target path for %s: %w", h.ID(), err)
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		logger.Error(err, "Failed to get control node from TaskContext")
		return nil, fmt.Errorf("failed to get control node for resource %s: %w", h.ID(), err)
	}

	// Path to the item that will be downloaded (archive or direct binary)
	// This path comes from util.GetBinaryInfo -> binaryInfo.FilePath
	downloadedItemPath := h.binaryInfo.FilePath
	downloadedItemDir := h.binaryInfo.ComponentDir // Directory where the item will be downloaded (e.g. .../etcd/v3.5.9/amd64/)

	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID
	lastNodeID := plan.NodeID("")

	runner := ctx.GetRunner()
	localConn, err := ctx.GetConnectorForHost(controlNode)
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for control node during pre-check: %w", err)
	}

	// --- Pre-check if final target binary already exists ---
	if exists, _ := runner.Exists(ctx.GoContext(), localConn, finalBinaryTargetPath); exists {
		logger.Info("Final target binary already exists. Assuming valid and skipping resource acquisition.", "path", finalBinaryTargetPath)
		return task.NewEmptyFragment(), nil
	}

	// --- Pre-check for downloaded item (archive or direct binary itself) if checksum is specified ---
	if h.ExpectedChecksum != "" {
		if itemExists, _ := runner.Exists(ctx.GoContext(), localConn, downloadedItemPath); itemExists {
			logger.Info("Downloaded item found locally, verifying checksum.", "item_path", downloadedItemPath)

			checksumAlgo := h.ChecksumAlgorithm
			if checksumAlgo == "" { checksumAlgo = "sha256" } // Default to sha256 if not specified

			// For pre-check, directly verify. If fails, proceed to download.
			var currentChecksum string
			var checksumErr error
			if strings.ToLower(checksumAlgo) == "sha256" {
				currentChecksum, checksumErr = runner.GetSHA256(ctx.GoContext(), localConn, downloadedItemPath)
			} else {
				logger.Warn("Unsupported checksum algorithm for pre-check, skipping direct verification.", "algo", checksumAlgo)
				// Fall through, download step will handle its own checksum if supported
			}

			if checksumErr == nil && currentChecksum == h.ExpectedChecksum {
				logger.Info("Local downloaded item checksum matches.")
				// Do not return empty fragment yet, still need to check extraction/finalization.
			} else if checksumErr != nil {
				logger.Warn("Failed to get checksum for local item, will re-download.", "item_path", downloadedItemPath, "error", checksumErr.Error())
			} else { // Checksum mismatch
				logger.Warn("Local item checksum mismatch, will re-download.", "item_path", downloadedItemPath, "expected", h.ExpectedChecksum, "got", currentChecksum)
			}
		}
	}

	// 1. Download Step (for the archive or direct binary)
	downloadNodeID := plan.NodeID(fmt.Sprintf("download-%s", h.ID()))
	downloadStep := commonstep.NewDownloadFileStep(
		"", // Instance name (can be auto-generated by step)
		h.binaryInfo.URL,
		downloadedItemPath, // Full path to save the downloaded file
		h.ExpectedChecksum,
		h.ChecksumAlgorithm,
		false, // Sudo (false for downloads to work_dir on control node)
	)
	nodes[downloadNodeID] = &plan.ExecutionNode{
		Name:     fmt.Sprintf("Download %s (%s)", h.ComponentName, h.binaryInfo.FileName),
		Step:     downloadStep,
		Hosts:    []connector.Host{controlNode},
		StepName: downloadStep.Meta().Name,
	}
	entryNodes = append(entryNodes, downloadNodeID)
	lastNodeID = downloadNodeID

	if h.binaryInfo.IsArchive {
		// Archive needs extraction
		archiveBase := strings.TrimSuffix(h.binaryInfo.FileName, filepath.Ext(h.binaryInfo.FileName))
		if strings.HasSuffix(archiveBase, ".tar") {
			archiveBase = strings.TrimSuffix(archiveBase, ".tar")
		}
		extractionDir := filepath.Join(downloadedItemDir, "extracted_"+archiveBase)

		extractNodeID := plan.NodeID(fmt.Sprintf("extract-%s", h.ID()))
		extractStep := commonstep.NewExtractArchiveStep(
			"",                 // Instance name
			downloadedItemPath, // Source is the downloaded archive
			extractionDir,      // Destination for extracted content
			false,              // RemoveArchiveAfterExtract
			false,              // Sudo
		)
		nodes[extractNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Extract %s archive", h.ComponentName),
			Step:         extractStep,
			Hosts:        []connector.Host{controlNode},
			Dependencies: []plan.NodeID{lastNodeID},
			StepName:     extractStep.Meta().Name,
		}
		lastNodeID = extractNodeID

		if h.BinaryNameInArchive == "" {
			// If BinaryNameInArchive is empty for an archive, it means the "product" of this handle
			// might be the entire extracted directory, or this was an oversight.
			// For now, the exit node is the extraction itself. Path() would point to the archive.
			logger.Info("Archive extracted. BinaryNameInArchive is empty, final product considered the extraction directory or archive.", "extraction_dir", extractionDir)
			exitNodes = append(exitNodes, lastNodeID)
		} else {
			// A specific binary from the archive is the target
			sourceBinaryInExtraction := filepath.Join(extractionDir, h.BinaryNameInArchive)

			// Ensure finalBinaryTargetPath's directory exists before copying
			ensureDirCmd := fmt.Sprintf("mkdir -p %s", filepath.Dir(finalBinaryTargetPath))
			ensureDirNodeID := plan.NodeID(fmt.Sprintf("ensure-finaldir-%s", h.ID()))
			ensureDirStep := command.NewCommandStep("",ensureDirCmd, false, false, 0, nil, 0, "",false,0,"",false) // Sudo false for control node workdir
			nodes[ensureDirNodeID] = &plan.ExecutionNode{
				Name: fmt.Sprintf("Ensure final directory for %s", h.ComponentName),
				Step: ensureDirStep,
				Hosts: []connector.Host{controlNode},
				Dependencies: []plan.NodeID{lastNodeID}, // Depends on extraction
				StepName: ensureDirStep.Meta().Name,
			}
			lastNodeID = ensureDirNodeID

			// Copy and make executable
			cmdToFinalize := fmt.Sprintf("cp -fp %s %s && chmod +x %s",
				sourceBinaryInExtraction,
				finalBinaryTargetPath,
				finalBinaryTargetPath,
			)
			finalizeNodeID := plan.NodeID(fmt.Sprintf("finalize-binary-%s", h.ID()))
			finalizeStep := command.NewCommandStep("", cmdToFinalize, false, false, 0, nil, 0, "", false, 0, "", false) // Sudo false
			nodes[finalizeNodeID] = &plan.ExecutionNode{
				Name:         fmt.Sprintf("Finalize %s binary", h.ComponentName),
				Step:         finalizeStep,
				Hosts:        []connector.Host{controlNode},
				Dependencies: []plan.NodeID{lastNodeID},
				StepName:     finalizeStep.Meta().Name,
			}
			exitNodes = append(exitNodes, finalizeNodeID)
		}
	} else {
		// Not an archive, the downloaded file (downloadedItemPath) is the target binary.
		// It needs to be at finalBinaryTargetPath and be executable.
		// Note: h.Path() for non-archives already points to downloadedItemPath (h.binaryInfo.FilePath).
		// So, finalBinaryTargetPath == downloadedItemPath in this case.

		chmodCmd := fmt.Sprintf("chmod +x %s", finalBinaryTargetPath)
		chmodNodeID := plan.NodeID(fmt.Sprintf("chmod-direct-binary-%s", h.ID()))
		chmodStep := command.NewCommandStep("", chmodCmd, false, false, 0, nil, 0, "", false, 0, "", false) // Sudo false
		nodes[chmodNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Chmod direct %s binary", h.ComponentName),
			Step:         chmodStep,
			Hosts:        []connector.Host{controlNode},
			Dependencies: []plan.NodeID{lastNodeID}, // Depends on download
			StepName:     chmodStep.Meta().Name,
		}
		exitNodes = append(exitNodes, chmodNodeID)
	}

	logger.Info("Remote binary resource assurance plan created.", "target_path_at_end_of_plan", finalBinaryTargetPath)
	return &task.ExecutionFragment{
		Nodes:      nodes,
		EntryNodes: entryNodes,
		ExitNodes:  exitNodes,
	}, nil
}


// Type returns the type of this resource handle.
func (h *RemoteBinaryHandle) Type() string {
	return "remote-binary"
}

// Ensure RemoteBinaryHandle implements the Handle interface.
var _ Handle = (*RemoteBinaryHandle)(nil)
