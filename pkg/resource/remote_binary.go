package resource

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common" // For ControlNodeHostName
	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host (though steps take it)
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	stepCommon "github.com/mensylisir/kubexm/pkg/step/common"
	stepComponentDownloads "github.com/mensylisir/kubexm/pkg/step/component_downloads" // Assuming a generic download step might live here or common
)

// RemoteBinaryHandle represents a resource that is a specific binary file
// expected to be found within an archive downloaded from a URL.
// The acquisition and preparation (download, extract) happen on the control node.
type RemoteBinaryHandle struct {
	// User-defined properties
	ResourceName        string // e.g., "etcd", "kubelet" - for naming steps and IDs
	Version             string // e.g., "v3.5.0", "v1.23.5"
	Arch                string // e.g., "amd64", "arm64" - if empty, might be derived from control-node facts
	DownloadURLTemplate string // Go template for the download URL, vars: {{.Version}}, {{.Arch}}, {{.FileName}}
	ArchiveFileName     string // Optional: If not provided, derived from URL or common patterns. E.g. "etcd-{{.Version}}-linux-{{.Arch}}.tar.gz"
	BinaryPathInArchive string // Relative path of the target binary within the extracted archive. E.g., "etcd-v3.5.0-linux-amd64/etcd"
	Checksum            string // Optional: Expected checksum of the downloaded archive (e.g., "sha256:value")
	// SudoForDownloadExtract bool // If download/extract steps on control node need sudo (rare for user work dirs)

	// Internal / Determined
	controlNodeHost []connector.Host // Cached control node host object
}

// NewRemoteBinaryHandle creates a new remote binary resource handle.
func NewRemoteBinaryHandle(
	resourceName, version, arch, urlTemplate, archiveFileName, binaryPathInArchive, checksum string,
) Handle {
	return &RemoteBinaryHandle{
		ResourceName:        resourceName,
		Version:             version,
		Arch:                arch,
		DownloadURLTemplate: urlTemplate,
		ArchiveFileName:     archiveFileName,
		BinaryPathInArchive: binaryPathInArchive,
		Checksum:            checksum,
		// SudoForDownloadExtract: sudo,
	}
}

func (h *RemoteBinaryHandle) ID() string {
	return fmt.Sprintf("%s-%s-%s-binary", h.ResourceName, h.Version, h.Arch)
}

// getControlNode retrieves and caches the control node host object.
func (h *RemoteBinaryHandle) getControlNode(ctx runtime.TaskContext) ([]connector.Host, error) {
	if h.controlNodeHost != nil {
		return h.controlNodeHost, nil
	}
	nodes, err := ctx.GetHostsByRole(common.ControlNodeRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for resource %s: %w", h.ID(), err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no control node found for resource %s", h.ID())
	}
	h.controlNodeHost = []connector.Host{nodes[0]} // Use the first control node
	return h.controlNodeHost, nil
}

// getDerivedArch determines the architecture, from spec or control-node facts.
func (h *RemoteBinaryHandle) getDerivedArch(ctx runtime.TaskContext, cnHost connector.Host) (string, error) {
	if h.Arch != "" {
		return h.Arch, nil
	}
	facts, err := ctx.GetHostFacts(cnHost)
	if err != nil {
		return "", fmt.Errorf("failed to get control node facts for arch auto-detection for resource %s: %w", h.ID(), err)
	}
	if facts.OS == nil || facts.OS.Arch == "" {
		return "", fmt.Errorf("control node OS.Arch is empty, cannot auto-detect arch for resource %s", h.ID())
	}
	// Normalize common variations
	derivedArch := facts.OS.Arch
	if derivedArch == "x86_64" {
		derivedArch = "amd64"
	} else if derivedArch == "aarch64" {
		derivedArch = "arm64"
	}
	return derivedArch, nil
}


// Path returns the expected final local path of the target binary on the control node.
func (h *RemoteBinaryHandle) Path(ctx runtime.TaskContext) string {
	// Path should be within a well-defined local work/cache area for this resource.
	// Example: <global_work_dir>/resources/<resource_id>/<binary_path_in_archive>
	// The GlobalWorkDir is from the main runtime context, accessible via TaskContext.

	// Arch might need to be determined first if not set.
	// This is a slight chicken-and-egg if Path() is called before EnsurePlan() might determine it.
	// For simplicity, assume Arch is determined or can be defaulted if Path() is called independently.
	// However, EnsurePlan will definitively set it.
	arch := h.Arch
	if arch == "" {
		// Attempt to get from cached control node facts if possible, or default.
		// This is best-effort if Path is called without EnsurePlan.
		// A robust way is EnsurePlan sets up internal determinedArch.
		// For now, let's assume if arch is empty here, it's problematic or needs a default.
		// This part is tricky, as Path() should be callable without running a plan.
		// Let's assume for now Arch is pre-filled or derived by the caller if needed for Path().
		// Or, Path() could also take the controlNode and try to derive arch.
		// A better approach: the *constructor* or an init method should determine arch.
		// For this example, we'll assume h.Arch is populated before Path() is called,
		// or the user accepts that an empty h.Arch might lead to an incomplete path.
	}

	baseResourceDir := filepath.Join(ctx.GetGlobalWorkDir(), "resources", h.ID())
	return filepath.Join(baseResourceDir, "extracted", h.BinaryPathInArchive)
}

// EnsurePlan generates the steps to download and extract the remote binary archive.
func (h *RemoteBinaryHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_handle", h.ID())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID

	controlNodeHosts, err := h.getControlNode(ctx)
	if err != nil {
		return nil, err
	}
	controlNode := controlNodeHosts[0] // We operate on a single control node

	// Determine architecture if not specified by user
	derivedArch, err := h.getDerivedArch(ctx, controlNode)
	if err != nil {
		return nil, err // Cannot proceed without arch
	}
	// Use derivedArch for subsequent operations; h.Arch remains user input.
	// If h.Arch was empty, it's now effectively derivedArch for this plan.

	finalBinaryPath := h.Path(ctx) // This will use h.Arch (user input) or derivedArch if h.Arch was empty and Path() is smart.
	                               // For safety, Path() should use derivedArch if h.Arch is empty.
	                               // Let's refine Path to use derivedArch if h.Arch is empty.
	                               // Or, more simply, ensure h.Arch is populated before calling Path if it's used for dir structure.
	                               // For this plan, we use `derivedArch` for URL and archive naming.
	                               // The `h.ID()` uses `h.Arch`, so if `h.Arch` was empty, ID might be less specific.
	                               // This suggests `Arch` should be determined and set on the handle early.
	                               // Let's assume for this example, the `h.Arch` field is updated if it was empty.
	if h.Arch == "" {
		h.Arch = derivedArch // Update the handle's arch if it was auto-detected. This makes ID() and Path() consistent.
		logger.Info("Architecture auto-detected and set on handle", "arch", h.Arch)
	}


	// Check if the final binary already exists (simple pre-check)
	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(controlNode) // Should be LocalConnector
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for control node: %w", err)
	}
	exists, _ := runner.Exists(ctx.GoContext(), conn, finalBinaryPath)
	if exists {
		// TODO: Add checksum verification for the existing binary if a binary checksum is provided
		logger.Info("Target binary already exists locally on control node. Skipping resource acquisition.", "path", finalBinaryPath)
		// Set output cache keys if this resource handle is expected to populate them
		// This part depends on how tasks consume the resource.
		// For now, just return an empty fragment.
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}

	// Define paths on the control node
	resourceWorkDir := filepath.Join(ctx.GetGlobalWorkDir(), "resources", h.ID())
	downloadTargetDir := filepath.Join(resourceWorkDir, "download")
	finalExtractDir := filepath.Join(resourceWorkDir, "extracted") // Where BinaryPathInArchive will be relative to

	// Determine archive filename and download URL
	archiveFileName := h.ArchiveFileName
	if archiveFileName == "" {
		archiveFileName = fmt.Sprintf("%s-%s-linux-%s.tar.gz", h.ResourceName, h.Version, h.Arch) // Common pattern
	}
	// Simple template replacement for URL
	downloadURL := strings.ReplaceAll(h.DownloadURLTemplate, "{{.Version}}", h.Version)
	downloadURL = strings.ReplaceAll(downloadURL, "{{.Arch}}", h.Arch) // Use the now determined Arch
	downloadURL = strings.ReplaceAll(downloadURL, "{{.FileName}}", archiveFileName)

	downloadedArchivePathKey := fmt.Sprintf("resource.%s.downloadedPath", h.ID())
	extractedArchiveDirKey := fmt.Sprintf("resource.%s.extractedPath", h.ID()) // This handle outputs dir

	// Node 1: Download the archive
	nodeIDDownload := plan.NodeID(fmt.Sprintf("resource-download-%s", h.ID()))
	// Using a generic DownloadFileStep from common steps, assuming it exists and is refactored.
	// If component_downloads.NewDownloadFileStep is more appropriate:
	// downloadStep := stepComponentDownloads.NewDownloadFileStep(
	//     fmt.Sprintf("Download-%s", h.ResourceName),
	//     downloadURL,
	//     downloadTargetDir, // Directory to download into
	//     archiveFileName,   // Specific filename
	//     h.Checksum,
	//     false, // sudo for download on control node (usually false for workdir)
	//     downloadedArchivePathKey, // Output key for the full path of the downloaded file
	// )
	// For now, let's assume a generic download step that takes full dest path.
	// The step component_downloads.DownloadContainerdStep etc. were specific.
	// We need a truly generic download step, or adapt one.
	// Let's use the structure of DownloadContainerdStep as a template for a generic one.
	// For this, we'll assume a conceptual `generic_download.NewDownloadFileStep`
	// Constructor: instanceName, url, downloadDir (on host), fileName (on host), checksum, sudo, outputCacheKey
	downloadStep := stepComponentDownloads.NewGenericDownloadStep( // Assuming this generic step exists
		fmt.Sprintf("Download-%s-Archive", h.ResourceName),
		downloadURL,
		downloadTargetDir, // Directory where file will be placed
		archiveFileName,   // Name of the file once downloaded
		h.Checksum,
		false,             // Sudo for download on control node usually false
		downloadedArchivePathKey,
	)
	nodes[nodeIDDownload] = &plan.ExecutionNode{
		Name:  fmt.Sprintf("Download %s archive", h.ResourceName),
		Step:  downloadStep,
		Hosts: controlNodeHosts,
	}
	entryNodes = append(entryNodes, nodeIDDownload)

	// Node 2: Extract the archive
	nodeIDExtract := plan.NodeID(fmt.Sprintf("resource-extract-%s", h.ID()))
	extractStep := stepCommon.NewExtractArchiveStep(
		fmt.Sprintf("Extract-%s-Archive", h.ResourceName),
		downloadedArchivePathKey,  // Input: path from download step's cache output
		finalExtractDir,           // Output: directory where archive is extracted
		extractedArchiveDirKey,    // Output: cache key for the extraction path (same as finalExtractDir)
		"",                        // ArchiveType (let step infer)
		false,                     // Sudo for extract on control node (usually false for workdir)
		false,                     // Preserve original archive (can be true if needed)
		true,                      // Remove extracted on rollback
	)
	nodes[nodeIDExtract] = &plan.ExecutionNode{
		Name:         fmt.Sprintf("Extract %s archive", h.ResourceName),
		Step:         extractStep,
		Hosts:        controlNodeHosts,
		Dependencies: []plan.NodeID{nodeIDDownload},
	}
	// The final path to the binary is h.Path(), which is based on finalExtractDir.
	// The extraction step makes `finalExtractDir` available.
	exitNodes = append(exitNodes, nodeIDExtract)

	return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
}

// Ensure RemoteBinaryHandle implements the Handle interface.
var _ Handle = (*RemoteBinaryHandle)(nil)

// --- GenericDownloadStep (Conceptual Placeholder) ---
// This is a placeholder for a generic download step that would be needed by RemoteBinaryHandle.
// Its actual implementation would be similar to the refactored component_download steps.

type GenericDownloadStep struct {
	step.NoOpStep // Embed NoOpStep to satisfy interface initially
	meta           spec.StepMeta
	URL            string
	DownloadDir    string // Target directory on the host
	FileName       string // Target filename on the host
	Checksum       string
	Sudo           bool
	OutputCacheKey string
}

func NewGenericDownloadStep(instanceName, url, downloadDir, fileName, checksum string, sudo bool, outputKey string) step.Step {
	return &GenericDownloadStep{
		meta: spec.StepMeta{
			Name:        instanceName,
			Description: fmt.Sprintf("Downloads file from %s to %s/%s", url, downloadDir, fileName),
		},
		URL:            url,
		DownloadDir:    downloadDir,
		FileName:       fileName,
		Checksum:       checksum,
		Sudo:           sudo,
		OutputCacheKey: outputKey,
	}
}

func (s *GenericDownloadStep) Meta() *spec.StepMeta { return &s.meta }

func (s *GenericDownloadStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, _ := ctx.GetConnectorForHost(host) // Error handling omitted for brevity
	facts, _ := ctx.GetHostFacts(host)       // Error handling omitted

	destinationPath := filepath.Join(s.DownloadDir, s.FileName)

	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.DownloadDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create download dir %s: %w", s.DownloadDir, err)
	}

	logger.Info("Downloading", "url", s.URL, "to", destinationPath)
	err := runnerSvc.Download(ctx.GoContext(), conn, facts, s.URL, destinationPath, s.Sudo)
	if err != nil {
		return err
	}
	if s.Checksum != "" {
		checksumValue := s.Checksum
		checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
		}
		if strings.ToLower(checksumType) == "sha256" {
			actualHash, errC := runnerSvc.GetSHA256(ctx.GoContext(), conn, destinationPath)
			if errC != nil {
				return fmt.Errorf("failed to get checksum for %s: %w", destinationPath, errC)
			}
			if !strings.EqualFold(actualHash, checksumValue) {
				return fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", destinationPath, checksumValue, actualHash)
			}
			logger.Info("Checksum verified", "path", destinationPath)
		} else {
			logger.Warn("Unsupported checksum type for verification during generic download", "type", checksumType)
		}
	}
	if s.OutputCacheKey != "" {
		ctx.TaskCache().Set(s.OutputCacheKey, destinationPath)
	}
	return nil
}
func (s *GenericDownloadStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	destinationPath := filepath.Join(s.DownloadDir, s.FileName)
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, destinationPath)
	if err != nil {
		logger.Warn("Failed to check file existence, proceeding with download.", "path", destinationPath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("File does not exist, download required.", "path", destinationPath)
		return false, nil
	}
	logger.Info("File already exists.", "path", destinationPath)

	if s.Checksum != "" {
		checksumValue := s.Checksum
		checksumType := "sha256" // Defaulting to sha256 for GetSHA256
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
		}
		if strings.ToLower(checksumType) != "sha256" {
			logger.Warn("Unsupported checksum type for precheck, only sha256 supported by GetSHA256. Skipping checksum validation.", "type", checksumType)
		} else {
			actualHash, errC := runnerSvc.GetSHA256(ctx.GoContext(), conn, destinationPath)
			if errC != nil {
				logger.Warn("Failed to get checksum for existing file, will re-download.", "path", destinationPath, "error", errC)
				return false, nil
			}
			if !strings.EqualFold(actualHash, checksumValue) {
				logger.Warn("Checksum mismatch for existing file. Will re-download.", "path", destinationPath, "expected", checksumValue, "actual", actualHash)
				// Optionally remove the existing file: runnerSvc.Remove(ctx.GoContext(), conn, destinationPath, s.Sudo)
				return false, nil
			}
			logger.Info("Checksum verified for existing file.", "path", destinationPath)
		}
	}

	// File exists and checksum matches (or no checksum specified)
	if s.OutputCacheKey != "" {
		ctx.TaskCache().Set(s.OutputCacheKey, destinationPath)
	}
	return true, nil
}

func (s *GenericDownloadStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for rollback.", "error", err)
		return nil // Best effort
	}

	destinationPath := filepath.Join(s.DownloadDir, s.FileName)
	logger.Info("Attempting to remove downloaded file for rollback.", "path", destinationPath)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, destinationPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove file during rollback (best effort).", "path", destinationPath, "error", err)
	} else {
		logger.Info("Successfully removed file if it existed.", "path", destinationPath)
	}
	if s.OutputCacheKey != "" {
		ctx.TaskCache().Delete(s.OutputCacheKey)
	}
	return nil
}

var _ step.Step = (*GenericDownloadStep)(nil)
// --- End GenericDownloadStep Placeholder ---
