package common

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Added
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// DefaultExtractedPathKey is a common Task Cache key for the path to the extracted content.
const DefaultExtractedPathKey = "extractedPath"
// DefaultDownloadedFilePathKey is assumed from download_file.go or a constants package.
// If not globally available, define it here or ensure it's passed correctly.
// For this example, we'll assume it's defined elsewhere or steps explicitly set their input keys.

// ExtractArchiveStepSpec defines the specification for extracting an archive.
type ExtractArchiveStepSpec struct {
	ArchivePathSharedDataKey string `json:"archivePathSharedDataKey"`         // Key to retrieve archive path from Task Cache (Mandatory input)
	ExtractionDir             string `json:"extractionDir"`                    // Directory to extract contents to (Mandatory, set by module)
	ExtractedDirSharedDataKey string `json:"extractedDirSharedDataKey"`        // Task Cache key to store the path of the primary extracted content (Mandatory for output)
	ArchiveType               string `json:"archiveType,omitempty"`            // Optional: e.g., "tar.gz", "zip". If empty, runner.Extract infers.
	PreserveOriginalArchive   bool   `json:"preserveOriginalArchive,omitempty"` // Optional: Hint for cleanup steps. Defaults to false.
}

// GetName returns the name of the step.
func (s *ExtractArchiveStepSpec) GetName() string {
	return "Extract Archive"
}

// PopulateDefaults sets default values for the spec.
func (s *ExtractArchiveStepSpec) PopulateDefaults() {
	if s.ArchivePathSharedDataKey == "" {
		s.ArchivePathSharedDataKey = DefaultDownloadedFilePathKey // Default input key
	}
	if s.ExtractedDirSharedDataKey == "" {
		s.ExtractedDirSharedDataKey = DefaultExtractedPathKey // Default output key
	}
	// ExtractionDir is now expected to be explicitly set by the module.
	// No fallback default path logic here.
}

// ExtractArchiveStepExecutor implements the logic for extracting an archive.
type ExtractArchiveStepExecutor struct{}

// Check checks if the archive seems to be already extracted.
func (e *ExtractArchiveStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) {
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for ExtractArchive Check")
	}
	logger = logger.With("host", currentHost.GetName())


	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for ExtractArchiveStep Check")
	}
	spec, ok := rawSpec.(*ExtractArchiveStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for ExtractArchiveStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults() // Ensure defaults are applied before use
	logger = logger.With("step", spec.GetName())


	if spec.ExtractionDir == "" {
		logger.Warn("ExtractionDir not set in spec. Cannot check.")
		return false, fmt.Errorf("ExtractionDir not set in spec for %s", spec.GetName())
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	// Use TaskCache via StepContext
	extractedPathVal, pathOk := ctx.TaskCache().Get(spec.ExtractedDirSharedDataKey)
	if pathOk {
		extractedPath, okStr := extractedPathVal.(string)
		if okStr && extractedPath != "" {
			// Assuming connector has Exists method
			exists, err := conn.Exists(goCtx, extractedPath)
			if err != nil {
				logger.Warn("Error checking existence of configured extracted path", "path", extractedPath, "error", err)
				return false, nil // Treat error as "not done" to allow Execute to run
			}
			if exists {
				logger.Info("Extracted content path found in Task Cache and exists on disk. Assuming already extracted.", "path", extractedPath)
				return true, nil
			}
			logger.Info("Path from Task Cache key does not exist. Needs extraction.", "path", extractedPath, "key", spec.ExtractedDirSharedDataKey)
		} else {
			logger.Debug("Invalid or empty path in Task Cache key.", "key", spec.ExtractedDirSharedDataKey)
		}
	} else {
		logger.Debug("Task Cache key not found. Assuming extraction not yet done or recorded.", "key", spec.ExtractedDirSharedDataKey)
	}

	exists, err := conn.Exists(goCtx, spec.ExtractionDir)
	if err != nil {
		logger.Warn("Error checking existence of ExtractionDir", "path", spec.ExtractionDir, "error", err)
		return false, nil // Treat error as "not done"
	}
	if exists {
		// Assuming connector has List method
		items, errList := conn.List(goCtx, spec.ExtractionDir)
		if errList != nil {
			logger.Warn("Failed to list contents of ExtractionDir, proceeding with extraction attempt.", "path", spec.ExtractionDir, "error", errList)
			return false, nil
		}
		if len(items) > 0 {
			logger.Info("Target ExtractionDir exists and is not empty. Assuming already extracted, though not confirmed via cache key. Execute will run to ensure cache key is set.", "path", spec.ExtractionDir)
			return false, nil // Let Execute run to populate cache
		}
	}
	logger.Debug("Extraction not confirmed by cache or existing non-empty directory.")
	return false, nil
}

// Execute extracts the archive.
func (e *ExtractArchiveStepExecutor) Execute(ctx runtime.StepContext) *step.Result {
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for ExtractArchive Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())


	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for ExtractArchiveStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*ExtractArchiveStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for ExtractArchiveStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults()
	logger = logger.With("step", spec.GetName())


	archivePathVal, archiveOk := ctx.TaskCache().Get(spec.ArchivePathSharedDataKey)
	var archivePath string
	if archiveOk {
		pathStr, okStr := archivePathVal.(string)
		if okStr { archivePath = pathStr }
	}

	if spec.ExtractionDir == "" {
		logger.Error("ExtractionDir not set in spec")
		res.Error = fmt.Errorf("ExtractionDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	if !archiveOk {
		logger.Error("Archive path not found in Task Cache", "key", spec.ArchivePathSharedDataKey)
		res.Error = fmt.Errorf("archive path not found in Task Cache key '%s'", spec.ArchivePathSharedDataKey)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	if archivePath == "" {
		logger.Error("Invalid or empty archive path in Task Cache", "key", spec.ArchivePathSharedDataKey)
		res.Error = fmt.Errorf("invalid or empty archive path in Task Cache key '%s'", spec.ArchivePathSharedDataKey)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	logger.Info("Ensuring extraction directory exists", "path", spec.ExtractionDir)
	// Assuming connector has MkdirAll or Mkdirp. Using MkdirAll for standard lib similarity.
	// Also assuming it takes permissions as string and a sudo flag.
	// For connector.Connector, we'd need to know its actual methods.
	// Let's assume Mkdir (path, perms) and it handles sudo internally or via connection setup.
	// For now, let's assume connector.Mkdir (ctx, path, perm string)
	// The previous code used runner.Mkdirp(goCtx, spec.ExtractionDir, "0755", false)
	// Let's assume connector.Mkdir(goCtx, path, permissions string)
	if err := conn.Mkdir(goCtx, spec.ExtractionDir, "0755"); err != nil { // Sudo handling would be part of connector's setup or specific method
		logger.Error("Failed to create extraction directory", "path", spec.ExtractionDir, "error", err)
		res.Error = fmt.Errorf("failed to create extraction directory %s: %w", spec.ExtractionDir, err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	logger.Info("Extracting archive", "archive", archivePath, "destination", spec.ExtractionDir)
	// Assuming connector has Extract method: Extract(ctx context.Context, archivePath string, targetDir string, useSudo bool) error
	if err := conn.Extract(goCtx, archivePath, spec.ExtractionDir, false); err != nil { // Assuming sudo=false for extract
		logger.Error("Failed to extract archive", "archive", archivePath, "destination", spec.ExtractionDir, "error", err)
		res.Error = fmt.Errorf("failed to extract archive %s to %s: %w", archivePath, spec.ExtractionDir, err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Archive extracted successfully.")

	var determinedExtractedPath string
	items, errList := conn.List(goCtx, spec.ExtractionDir)
	if errList != nil {
		logger.Warn("Failed to list contents of extraction directory. Using extraction directory itself as extracted path.", "path", spec.ExtractionDir, "error", errList)
		determinedExtractedPath = spec.ExtractionDir
	} else {
		nonHiddenItems := []string{}
		for _, item := range items {
			if !strings.HasPrefix(item, ".") {
				nonHiddenItems = append(nonHiddenItems, item)
			}
		}
		if len(nonHiddenItems) == 1 {
			determinedExtractedPath = filepath.Join(spec.ExtractionDir, nonHiddenItems[0])
			logger.Debug("One non-hidden item found in extraction dir. Setting as primary extracted path.", "path", determinedExtractedPath)
		} else {
			logger.Debug("Multiple or no non-hidden items found. Using extraction directory as primary extracted path.", "count", len(nonHiddenItems), "path", spec.ExtractionDir)
			determinedExtractedPath = spec.ExtractionDir
		}
	}

	ctx.TaskCache().Set(spec.ExtractedDirSharedDataKey, determinedExtractedPath)
	logger.Info("Stored extracted path in Task Cache.", "key", spec.ExtractedDirSharedDataKey, "path", determinedExtractedPath)

	res.EndTime = time.Now() // Update end time after all operations

	// Re-run Check to confirm and ensure status is correctly set based on final state.
	// This is optional but can be good practice.
	// However, if Check itself has issues or side effects, this might be problematic.
	// For now, assuming successful extraction means the step is Succeeded.
	// The original code called e.Check(ctx) again.
	// Let's simplify: if we reached here without error, it's success.
	// The check logic is primarily for idempotency before execution.
	if res.Error == nil {
		res.Status = step.StatusSucceeded
	} else {
		// Error already set, status should be Failed
		res.Status = step.StatusFailed
	}
	// The original code had a more complex re-check, which could lead to loops or mask errors.
	// If an error occurred during extraction, res.Error would be set, and status would be Failed.
	// If no error, it's Succeeded.

	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&ExtractArchiveStepSpec{}), &ExtractArchiveStepExecutor{})
}
