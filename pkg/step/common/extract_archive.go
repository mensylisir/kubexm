package common

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/spec"
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
func (e *ExtractArchiveStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for ExtractArchiveStep Check")
	}
	spec, ok := currentFullSpec.(*ExtractArchiveStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for ExtractArchiveStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())

	if spec.ExtractionDir == "" { // Module must provide this.
		logger.Warnf("ExtractionDir not set in spec for %s. Cannot check.", spec.GetName())
		return false, fmt.Errorf("ExtractionDir not set in spec for %s", spec.GetName())
	}

	extractedPathVal, pathOk := ctx.Task().Get(spec.ExtractedDirSharedDataKey)
	if pathOk {
		extractedPath, okStr := extractedPathVal.(string)
		if okStr && extractedPath != "" {
			// Check if the specific path stored (which could be a sub-folder or the extraction dir itself) exists.
			exists, err := ctx.Host.Runner.Exists(ctx.GoContext, extractedPath)
			if err != nil {
				logger.Warnf("Error checking existence of configured extracted path %s: %v", extractedPath, err)
				return false, nil
			}
			if exists {
				logger.Infof("Extracted content path %s found in Task Cache and exists on disk. Assuming already extracted.", extractedPath)
				return true, nil
			}
			logger.Infof("Path %s from Task Cache key %s does not exist. Needs extraction.", extractedPath, spec.ExtractedDirSharedDataKey)
		} else {
			logger.Debugf("Invalid or empty path in Task Cache key %s.", spec.ExtractedDirSharedDataKey)
		}
	} else {
		logger.Debugf("Task Cache key %s not found. Assuming extraction not yet done or recorded.", spec.ExtractedDirSharedDataKey)
	}

	// If ExtractedDirSharedDataKey is not in cache, check if the target ExtractionDir itself exists and is not empty.
	// This is a less precise check but can prevent re-extraction to the same explicit directory.
	exists, _ := ctx.Host.Runner.Exists(ctx.GoContext, spec.ExtractionDir)
	if exists {
		items, _ := ctx.Host.Runner.List(ctx.GoContext, spec.ExtractionDir)
		if len(items) > 0 {
			logger.Infof("Target ExtractionDir %s exists and is not empty. Assuming already extracted, though not confirmed via cache key.", spec.ExtractionDir)
			// To be truly "done" by this path, Execute should still run to populate the cache key.
			// So, return false to ensure Execute runs and sets the cache key.
			return false, nil
		}
	}
	return false, nil
}

// Execute extracts the archive.
func (e *ExtractArchiveStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for ExtractArchiveStep Execute"))
	}
	spec, ok := currentFullSpec.(*ExtractArchiveStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for ExtractArchiveStep Execute: %T", currentFullSpec))
	}

	archivePathVal, archiveOk := ctx.Task().Get(spec.ArchivePathSharedDataKey)
	var archivePath string
	if archiveOk {
		pathStr, okStr := archivePathVal.(string)
		if okStr { archivePath = pathStr }
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	if spec.ExtractionDir == "" { // Module must provide this.
		res.Error = fmt.Errorf("ExtractionDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; return res
	}
	if !archiveOk {
		res.Error = fmt.Errorf("archive path not found in Task Cache key '%s'", spec.ArchivePathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}
	if archivePath == "" {
		res.Error = fmt.Errorf("invalid or empty archive path in Task Cache key '%s'", spec.ArchivePathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}

	logger.Infof("Ensuring extraction directory %s exists...", spec.ExtractionDir)
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, spec.ExtractionDir, "0755", false); err != nil {
		res.Error = fmt.Errorf("failed to create extraction directory %s: %w", spec.ExtractionDir, err)
		res.Status = step.StatusFailed; return res
	}

	logger.Infof("Extracting archive %s to %s...", archivePath, spec.ExtractionDir)
	if err := ctx.Host.Runner.Extract(ctx.GoContext, archivePath, spec.ExtractionDir, false); err != nil {
		res.Error = fmt.Errorf("failed to extract archive %s to %s: %w", archivePath, spec.ExtractionDir, err)
		res.Status = step.StatusFailed; return res
	}
	logger.Successf("Archive %s extracted successfully to %s.", archivePath, spec.ExtractionDir)

	var determinedExtractedPath string
	items, err := ctx.Host.Runner.List(ctx.GoContext, spec.ExtractionDir)
	if err != nil {
		logger.Warnf("Failed to list contents of extraction directory %s: %v. Using extraction directory itself as extracted path.", spec.ExtractionDir, err)
		determinedExtractedPath = spec.ExtractionDir
	} else {
		// If archive extracts to a single sub-folder (e.g. etcd-v3.x.x-linux-amd64.tar.gz -> etcd-v3.x.x-linux-amd64/)
		// then determinedExtractedPath should be that sub-folder.
		// Otherwise, it's the ExtractionDir itself.
		nonHiddenItems := []string{}
		for _, item := range items {
			if !strings.HasPrefix(item, ".") { // Ignore hidden files/folders like .DS_Store
				nonHiddenItems = append(nonHiddenItems, item)
			}
		}
		if len(nonHiddenItems) == 1 {
			determinedExtractedPath = filepath.Join(spec.ExtractionDir, nonHiddenItems[0])
			logger.Debugf("One non-hidden item found in extraction dir: %s. Setting as primary extracted path.", determinedExtractedPath)
		} else {
			logger.Debugf("%d non-hidden items found in extraction dir (or empty). Using extraction directory %s as primary extracted path.", len(nonHiddenItems), spec.ExtractionDir)
			determinedExtractedPath = spec.ExtractionDir
		}
	}

	ctx.Task().Set(spec.ExtractedDirSharedDataKey, determinedExtractedPath)
	logger.Infof("Stored extracted path '%s' in Task Cache key '%s'.", determinedExtractedPath, spec.ExtractedDirSharedDataKey)

	done, checkErr := e.Check(ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed
		return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates archive extraction was not successful or path not recorded/found")
		res.Status = step.StatusFailed
		return res
	}
	res.Status = step.StatusSucceeded
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&ExtractArchiveStepSpec{}), &ExtractArchiveStepExecutor{})
}
