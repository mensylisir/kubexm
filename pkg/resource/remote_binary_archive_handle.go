package resource

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template" // For URL and path templating
	"bytes"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector" // For controlNode representation
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common" // Assuming DownloadFileStep and ExtractArchiveStep
)

// RemoteBinaryArchiveHandle represents a remote binary archive (e.g., .tar.gz)
// that contains one or more useful binaries.
type RemoteBinaryArchiveHandle struct {
	// LogicalName is a short name for the component, e.g., "etcd", "kubelet". Used for path generation.
	LogicalName string
	Version     string
	Arch        string // If empty, attempts to use host arch or a common default like amd64.
	// URLTemplate is a Go template string for the download URL.
	// It can use {{.Version}} and {{.Arch}}.
	URLTemplate string
	// ArchiveFileNameTemplate is a Go template string for the archive file name.
	// If empty, it will be derived from the URL. Can use {{.Version}} and {{.Arch}}.
	ArchiveFileNameTemplate string
	// BinariesInArchive is a map where keys are logical names for binaries (e.g., "etcd", "etcdctl")
	// and values are their relative paths within the extracted archive.
	// Path template can use {{.Version}} and {{.Arch}}.
	BinariesInArchive map[string]string
	Checksum          string // Expected SHA256 checksum of the archive file.
	ChecksumType      string // e.g., "sha256"
}

// NewRemoteBinaryArchiveHandle creates a new RemoteBinaryArchiveHandle.
func NewRemoteBinaryArchiveHandle(
	logicalName, version, arch, urlTemplate, archiveFileNameTemplate string,
	binariesInArchive map[string]string,
	checksum string,
) Handle { // Implements resource.Handle
	return &RemoteBinaryArchiveHandle{
		LogicalName:             logicalName,
		Version:                 version,
		Arch:                    arch,
		URLTemplate:             urlTemplate,
		ArchiveFileNameTemplate: archiveFileNameTemplate,
		BinariesInArchive:       binariesInArchive,
		Checksum:                checksum,
		ChecksumType:            "sha256", // Default, can be made configurable
	}
}

func (h *RemoteBinaryArchiveHandle) renderTemplate(tmplStr string, data interface{}) (string, error) {
	if tmplStr == "" {
		return "", nil
	}
	tmpl, err := template.New("render").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

func (h *RemoteBinaryArchiveHandle) getEffectiveArch(ctx runtime.TaskContext) string {
	if h.Arch != "" {
		return h.Arch
	}
	// Attempt to get arch from control node facts if available in TaskContext
	// This is a simplification; TaskContext might not easily provide control node arch.
	// For now, default to a common arch if not specified.
	// A better approach: RuntimeBuilder determines control node arch and stores it in context.
	// Or, Arch is mandatory for remote resources if not derivable.
	// TODO: Get control node arch from context. For now, default.
	logger := ctx.GetLogger().With("resource_id", h.ID())
	logger.Debug("Architecture not specified for remote binary archive, defaulting to amd64. Consider specifying Arch.")
	return "amd64"
}


// ID returns a unique identifier for this resource instance.
func (h *RemoteBinaryArchiveHandle) ID() string {
	// Arch part is tricky if it's auto-detected. For a stable ID, resolved Arch should be used.
	// This ID is used by tasks to refer to the handle, so it should be stable before EnsurePlan.
	// For now, use specified Arch or a placeholder if auto-detection is implied.
	archPart := h.Arch
	if archPart == "" {
		archPart = "auto" // Placeholder for auto-detected arch
	}
	return fmt.Sprintf("remote-archive-%s-%s-%s", h.LogicalName, h.Version, archPart)
}

// getPathComponents computes common path elements based on context and handle properties.
func (h *RemoteBinaryArchiveHandle) getPathComponents(ctx runtime.TaskContext) (baseWorkDir, componentDir, versionDir, archDir string, err error) {
	clusterName := ctx.GetClusterConfig().Name
	if clusterName == "" {
		return "", "", "", "", fmt.Errorf("cluster name is empty in configuration")
	}

	// Base directory for all downloads for this cluster on the control node
	// e.g., $(pwd)/.kubexm/mycluster
	baseWorkDir = filepath.Join(ctx.GetWorkDir(), common.DefaultWorkDirName, clusterName)

	componentDir = filepath.Join(baseWorkDir, h.LogicalName)
	versionDir = filepath.Join(componentDir, h.Version)

	effectiveArch := h.getEffectiveArch(ctx)
	archDir = filepath.Join(versionDir, effectiveArch)
	return baseWorkDir, componentDir, versionDir, archDir, nil
}

// archiveDownloadPath returns the local path on the control node where the archive should be/is downloaded.
func (h *RemoteBinaryArchiveHandle) archiveDownloadPath(ctx runtime.TaskContext) (string, error) {
	_, _, _, archDir, err := h.getPathComponents(ctx)
	if err != nil {
		return "", err
	}

	templateData := map[string]string{
		"Version": h.Version,
		"Arch":    h.getEffectiveArch(ctx),
	}
	archiveFileName := h.ArchiveFileNameTemplate
	if archiveFileName == "" {
		// Derive from URL if template is not provided
		url, errRenderURL := h.renderTemplate(h.URLTemplate, templateData)
		if errRenderURL != nil {
			return "", fmt.Errorf("failed to render URL to derive archive name: %w", errRenderURL)
		}
		archiveFileName = filepath.Base(url)
	} else {
		renderedName, errRender := h.renderTemplate(archiveFileName, templateData)
		if errRender != nil {
			return "", fmt.Errorf("failed to render archive file name template: %w", errRender)
		}
		archiveFileName = renderedName
	}
	if archiveFileName == "" {
		return "", fmt.Errorf("could not determine archive file name for %s", h.ID())
	}
	return filepath.Join(archDir, archiveFileName), nil
}

// extractedArchiveRootPath returns the local path on the control node where the archive is extracted.
func (h *RemoteBinaryArchiveHandle) extractedArchiveRootPath(ctx runtime.TaskContext) (string, error) {
	_, _, _, archDir, err := h.getPathComponents(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(archDir, "extracted"), nil
}

// Path returns the local path of a *specific binary* within the extracted archive on the control node.
// This is what tasks will use as a source for UploadFileStep etc.
// This method assumes EnsurePlan has already run successfully.
func (h *RemoteBinaryArchiveHandle) Path(binaryKeyName string, ctx runtime.TaskContext) (string, error) {
	relativePathInArchiveTmpl, ok := h.BinariesInArchive[binaryKeyName]
	if !ok {
		return "", fmt.Errorf("binary key '%s' not defined in BinariesInArchive for resource %s", binaryKeyName, h.ID())
	}

	templateData := map[string]string{
		"Version": h.Version,
		"Arch":    h.getEffectiveArch(ctx),
	}
	relativePathInArchive, err := h.renderTemplate(relativePathInArchiveTmpl, templateData)
	if err != nil {
		return "", fmt.Errorf("failed to render relative path for binary '%s': %w", binaryKeyName, err)
	}

	extractedRoot, err := h.extractedArchiveRootPath(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(extractedRoot, relativePathInArchive), nil
}

// Path (from resource.Handle interface) - for archives, this is ambiguous.
// Let's make it return the extracted root directory path.
// Tasks should use GetLocalPath(binaryName) for specific binaries.
func (h *RemoteBinaryArchiveHandle) Path(ctx runtime.TaskContext) string {
	path, err := h.extractedArchiveRootPath(ctx)
	if err != nil {
		// Path() interface method doesn't return error. Log it and return empty or base download dir.
		ctx.GetLogger().Error(err, "Failed to determine extracted archive root path for Path() method", "resource_id", h.ID())
		dlPath, _ := h.archiveDownloadPath(ctx) // Best effort
		return dlPath
	}
	return path
}


// GetLocalPath is a more specific method for RemoteBinaryArchiveHandle.
func (h *RemoteBinaryArchiveHandle) GetLocalPath(binaryKeyName string, ctx runtime.TaskContext) (string, error) {
	return h.Path(binaryKeyName, ctx)
}


// EnsurePlan generates an ExecutionFragment to download and extract the archive on the control node.
func (h *RemoteBinaryArchiveHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_id", h.ID())
	fragment := task.NewExecutionFragment()

	controlHost, err := ctx.GetControlNode() // Needs implementation in TaskContext
	if err != nil {
		return nil, fmt.Errorf("failed to get control node from context for resource %s: %w", h.ID(), err)
	}
	if controlHost == nil {
		return nil, fmt.Errorf("control node is nil in context for resource %s", h.ID())
	}

	downloadPath, err := h.archiveDownloadPath(ctx)
	if err != nil {
		return nil, err
	}
	extractedRootPath, err := h.extractedArchiveRootPath(ctx)
	if err != nil {
		return nil, err
	}

	// Check if final expected binaries exist (simplistic precheck for idempotency)
	// A more robust check would verify all binaries in h.BinariesInArchive
	// and potentially check checksums of extracted files if available.
	allBinariesExist := true
	if len(h.BinariesInArchive) > 0 {
		for binKey := range h.BinariesInArchive {
			finalBinPath, err := h.Path(binKey, ctx)
			if err != nil {
				logger.Warn("Could not determine path for binary, assuming it doesn't exist.", "binaryKey", binKey, "error", err)
				allBinariesExist = false; break
			}
			// This check needs a connector to the control node.
			// The runtime.TaskContext needs to provide a way to run commands/checks on the control node.
			// For now, assume a simplified check or that steps handle it.
			// This check should ideally use runner.Exists on the control node.
			// Let's assume a StepContext for control node can be derived or steps handle this.
			// For this stage, we'll rely on DownloadFileStep and ExtractArchiveStep's own prechecks.
			// If they are idempotent, the overall EnsurePlan becomes idempotent.
			// So, if `extractedRootPath` exists, we might assume extraction is done.
			// This is a simplification. A real check would be better.
			// For now, if extractedRootPath exists, assume done. This is not very robust.
			// A better check: if all `h.Path(binKey, ctx)` exist.
			// This requires a runner for the control node.
			// TODO: Implement proper precheck for existing extracted files.
			// For now, we rely on the idempotency of the download/extract steps.
			// This means EnsurePlan will always return download/extract steps,
			// and those steps' Precheck methods will determine if they need to run.
			allBinariesExist = false // Force re-evaluation by steps for now
		}
	} else {
		allBinariesExist = false // No binaries defined, so can't be "all exist"
	}


	if allBinariesExist {
		logger.Info("All target binaries from archive already exist in local extracted path. Skipping download/extraction.")
		// Populate cache with paths if not already there? Or assume steps do it.
		// For now, return empty fragment, assuming paths are discoverable/stable.
		return task.NewEmptyFragment(), nil
	}

	// --- Node 1: Download the archive ---
	templateData := map[string]string{"Version": h.Version, "Arch": h.getEffectiveArch(ctx)}
	downloadURL, err := h.renderTemplate(h.URLTemplate, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render download URL: %w", err)
	}

	downloadStepName := fmt.Sprintf("Download-%s-%s-%s", h.LogicalName, h.Version, h.getEffectiveArch(ctx))
	downloadStep := commonsteps.NewDownloadFileStep(
		downloadStepName,
		downloadURL,
		downloadPath,
		h.Checksum,     // Checksum for the downloaded file
		h.ChecksumType, // Checksum type
		false,          // Sudo not needed for downloads to user-writable workdir
	)
	downloadNodeID := plan.NodeID(downloadStepName)
	fragment.Nodes[downloadNodeID] = &plan.ExecutionNode{
		Name:         downloadStepName,
		Step:         downloadStep,
		Hosts:        []connector.Host{controlHost}, // Runs on control node
		StepName:     downloadStep.Meta().Name,
		Dependencies: []plan.NodeID{},
	}
	fragment.EntryNodes = append(fragment.EntryNodes, downloadNodeID)

	// --- Node 2: Extract the archive ---
	extractStepName := fmt.Sprintf("Extract-%s-%s-%s", h.LogicalName, h.Version, h.getEffectiveArch(ctx))
	// The ExtractArchiveStep needs to know the path to the archive (output of downloadStep)
	// and where to extract it. It should also cache the path to the actual extracted files.
	extractStep := commonsteps.NewExtractArchiveStep(
		extractStepName,
		downloadPath,      // Source archive path (from download step)
		extractedRootPath, // Destination directory for extraction
		true,              // RemoveArchiveAfterExtract
		false,             // Sudo not needed for extraction in workdir
		// This step should cache `extractedRootPath` or specific binary paths upon success.
		// For example, using `ctx.StepOutputCache().Set("ExtractedPath", extractedRootPath)`
	)
	extractNodeID := plan.NodeID(extractStepName)
	fragment.Nodes[extractNodeID] = &plan.ExecutionNode{
		Name:         extractStepName,
		Step:         extractStep,
		Hosts:        []connector.Host{controlHost}, // Runs on control node
		StepName:     extractStep.Meta().Name,
		Dependencies: []plan.NodeID{downloadNodeID},
	}
	fragment.ExitNodes = append(fragment.ExitNodes, extractNodeID)

	// The paths to individual binaries (e.g., etcd, etcdctl) are derived from extractedRootPath
	// and h.BinariesInArchive. The handle's Path(binaryKey, ctx) method does this.
	// The task using this handle will call Path() after this fragment is planned/executed.

	logger.Info("Planned steps for resource acquisition on control node.", "downloadNode", downloadNodeID, "extractNode", extractNodeID)
	return fragment, nil
}

// Ensure RemoteBinaryArchiveHandle implements Handle
var _ Handle = (*RemoteBinaryArchiveHandle)(nil)

[end of pkg/resource/remote_binary_archive_handle.go]
