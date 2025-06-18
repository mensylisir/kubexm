package common

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// Assuming DefaultDownloadedFilePathKey is defined in a constants package or similar
	// For this example, let's assume it's available or use the one from download_file.go
)

// DefaultExtractedPathKey is a common SharedData key for the path to the extracted content.
const DefaultExtractedPathKey = "extractedPath"

// ExtractArchiveStepSpec defines the specification for extracting an archive.
type ExtractArchiveStepSpec struct {
	ArchivePathSharedDataKey string `json:"archivePathSharedDataKey"`         // Key to retrieve archive path from SharedData (Mandatory)
	ExtractionDir             string `json:"extractionDir"`                    // Directory to extract contents to (Mandatory)
	ExtractedDirSharedDataKey string `json:"extractedDirSharedDataKey"`        // SharedData key to store the path of the primary extracted content (Mandatory for output)
	ArchiveType               string `json:"archiveType,omitempty"`            // Optional: e.g., "tar.gz", "zip". If empty, runner.Extract infers.
	PreserveOriginalArchive   bool   `json:"preserveOriginalArchive,omitempty"` // Optional: Hint for cleanup steps. Defaults to false.
}

// GetName returns the name of the step.
func (s *ExtractArchiveStepSpec) GetName() string {
	return "Extract Archive"
}

// PopulateDefaults sets default values for the spec.
func (s *ExtractArchiveStepSpec) PopulateDefaults(ctx *runtime.Context, archivePathFromSharedData string) {
	if s.ArchivePathSharedDataKey == "" {
		// Assuming DefaultDownloadedFilePathKey is available, e.g. from a constants package or defined in this package
		s.ArchivePathSharedDataKey = DefaultDownloadedFilePathKey // From download_file.go for example
	}
	if s.ExtractedDirSharedDataKey == "" {
		s.ExtractedDirSharedDataKey = DefaultExtractedPathKey
	}
	if s.ExtractionDir == "" {
		baseDir := "/tmp/kubexms_extracts"
		if ctx != nil && ctx.WorkDir != "" {
			baseDir = filepath.Join(ctx.WorkDir, "extracts")
		}
		// Create a unique extraction directory if not specified.
		// Using the base of the archive path can be problematic if archivePathFromSharedData is empty during PopulateDefaults.
		// A timestamp or a random string is safer for a default unique dir.
		s.ExtractionDir = filepath.Join(baseDir, fmt.Sprintf("extract-%d", time.Now().UnixNano()))
		if archivePathFromSharedData != "" {
			// A more descriptive name if archive path is known, e.g. extract-etcd-v3.5.0.tar.gz-<timestamp>
			archiveName := filepath.Base(archivePathFromSharedData)
			s.ExtractionDir = filepath.Join(baseDir, fmt.Sprintf("%s-extract-%d", strings.TrimSuffix(archiveName, filepath.Ext(archiveName)), time.Now().UnixNano()))
		}
	}
}

// ExtractArchiveStepExecutor implements the logic for extracting an archive.
type ExtractArchiveStepExecutor struct{}

// Check checks if the archive seems to be already extracted.
func (e *ExtractArchiveStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	stepSpec, ok := s.(*ExtractArchiveStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for %s", s, stepSpec.GetName())
	}
	// PopulateDefaults needs archivePath for a better default ExtractionDir.
	// However, archivePath is from SharedData, which might not be set when Check is first called.
	// So, pass an empty string for archive path to PopulateDefaults in Check.
	// The actual ExtractionDir used by Execute will be based on archivePath known at that time if default is used.
	stepSpec.PopulateDefaults(ctx, "") // archivePath not critical for Check's default ExtractionDir
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())

	// Primarily rely on ExtractedDirSharedDataKey being populated and the path existing.
	extractedPathVal, pathOk := ctx.SharedData.Load(stepSpec.ExtractedDirSharedDataKey)
	if pathOk {
		extractedPath, okStr := extractedPathVal.(string)
		if okStr && extractedPath != "" {
			exists, err := ctx.Host.Runner.Exists(ctx.GoContext, extractedPath)
			if err != nil {
				logger.Warnf("Error checking existence of configured extracted path %s: %v", extractedPath, err)
				return false, nil // Error during check, assume not done
			}
			if exists {
				// Additionally, one might check if the directory is empty or contains expected markers/files.
				// For now, existence of the path from SharedData is sufficient.
				logger.Infof("Extracted content path %s found in SharedData and exists on disk. Assuming already extracted.", extractedPath)
				return true, nil
			}
			logger.Infof("Path %s from SharedData key %s does not exist. Needs extraction.", extractedPath, stepSpec.ExtractedDirSharedDataKey)
		} else {
			logger.Debugf("Invalid or empty path in SharedData key %s.", stepSpec.ExtractedDirSharedDataKey)
		}
	} else {
		logger.Debugf("SharedData key %s not found. Assuming extraction not yet done or recorded.", stepSpec.ExtractedDirSharedDataKey)
	}

	// Fallback: if ExtractionDir is explicitly set and exists, consider it potentially done.
	// This is less reliable as ExtractionDir might exist from a failed previous attempt.
	// This part of check is mostly for cases where spec is re-used and ExtractionDir is fixed.
	if stepSpec.ExtractionDir != "" && !strings.Contains(stepSpec.ExtractionDir, "extract-") { // Avoid checking default timestamped dirs
	    exists, _ := ctx.Host.Runner.Exists(ctx.GoContext, stepSpec.ExtractionDir)
	    if exists {
	        // Check if it's not empty. A more robust check would look for specific files.
	        // This is a very basic check.
	        // entries, _ := ctx.Host.Runner.List(ctx.GoContext, stepSpec.ExtractionDir)
	        // if len(entries) > 0 {
	        //    logger.Infof("Explicit ExtractionDir %s exists and is not empty. Assuming already extracted.", stepSpec.ExtractionDir)
	        //    return true, nil
	        // }
	    }
	}

	return false, nil
}

// Execute extracts the archive.
func (e *ExtractArchiveStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*ExtractArchiveStepSpec)
	if !ok {
		return step.NewResultForSpec(s, fmt.Errorf("unexpected spec type %T", s))
	}
	// Get archive path from SharedData to inform default ExtractionDir if necessary
	archivePathVal, archiveOk := ctx.SharedData.Load(stepSpec.ArchivePathSharedDataKey)
	var archivePath string
	if archiveOk {
		pathStr, okStr := archivePathVal.(string)
		if okStr { archivePath = pathStr }
	}
	stepSpec.PopulateDefaults(ctx, archivePath) // Now call with potentially known archivePath

	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResultForSpec(s, nil)

	if !archiveOk {
		res.Error = fmt.Errorf("archive path not found in SharedData key '%s'", stepSpec.ArchivePathSharedDataKey)
		res.SetFailed(); return res
	}
	if archivePath == "" {
		res.Error = fmt.Errorf("invalid or empty archive path in SharedData key '%s'", stepSpec.ArchivePathSharedDataKey)
		res.SetFailed(); return res
	}

	logger.Infof("Ensuring extraction directory %s exists...", stepSpec.ExtractionDir)
	// Sudo for Mkdirp depends on where ExtractionDir is. For /tmp or WorkDir, usually false.
	// Let's assume false by default for extraction, or make it configurable if needed.
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, stepSpec.ExtractionDir, "0755", false); err != nil {
		res.Error = fmt.Errorf("failed to create extraction directory %s: %w", stepSpec.ExtractionDir, err)
		res.SetFailed(); return res
	}

	logger.Infof("Extracting archive %s to %s...", archivePath, stepSpec.ExtractionDir)
	// Sudo for Extract also depends on permissions of ExtractionDir.
	if err := ctx.Host.Runner.Extract(ctx.GoContext, archivePath, stepSpec.ExtractionDir, false); err != nil {
		res.Error = fmt.Errorf("failed to extract archive %s to %s: %w", archivePath, stepSpec.ExtractionDir, err)
		res.SetFailed(); return res
	}
	logger.Successf("Archive %s extracted successfully to %s.", archivePath, stepSpec.ExtractionDir)

	// Determine primary extracted content path
	var determinedExtractedPath string
	items, err := ctx.Host.Runner.List(ctx.GoContext, stepSpec.ExtractionDir) // Assumes Runner.List method exists
	if err != nil {
		logger.Warnf("Failed to list contents of extraction directory %s: %v. Using extraction directory itself as extracted path.", stepSpec.ExtractionDir, err)
		determinedExtractedPath = stepSpec.ExtractionDir
	} else {
		if len(items) == 1 {
			determinedExtractedPath = filepath.Join(stepSpec.ExtractionDir, items[0])
			logger.Debugf("One item found in extraction dir: %s. Setting as primary extracted path.", determinedExtractedPath)
		} else {
			logger.Debugf("%d items found in extraction dir. Using extraction directory %s as primary extracted path.", len(items), stepSpec.ExtractionDir)
			determinedExtractedPath = stepSpec.ExtractionDir // Default to the extraction dir itself if multiple items or empty
		}
	}

	ctx.SharedData.Store(stepSpec.ExtractedDirSharedDataKey, determinedExtractedPath)
	logger.Infof("Stored extracted path '%s' in SharedData key '%s'.", determinedExtractedPath, stepSpec.ExtractedDirSharedDataKey)

	// Perform post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed()
		return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates archive extraction was not successful or path not recorded")
		res.SetFailed()
		return res
	}

	res.SetSucceeded()
	return res
}

func init() {
	// Register the new generic step
	step.Register(&ExtractArchiveStepSpec{}, &ExtractArchiveStepExecutor{})
}
