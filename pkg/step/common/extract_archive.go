package common

import (
	"fmt"
	"path/filepath"
	"strings"
	// "time" // No longer directly used by this step's logic, timeouts handled by context/connector
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/spec"    // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/utils"   // For PathRequiresSudo or other path/archive utils
)

// ExtractArchiveStepSpec defines parameters for extracting an archive file.
type ExtractArchiveStepSpec struct {
	spec.StepMeta `json:",inline"`

	ArchivePath              string `json:"archivePath,omitempty"`              // Direct path to the archive file on the target host
	ArchivePathCacheKey      string `json:"archivePathCacheKey,omitempty"`  // Optional: Key to get archive path from cache
	ExtractionDir            string `json:"extractionDir,omitempty"`            // Directory where the archive should be extracted
	ArchiveType              string `json:"archiveType,omitempty"`              // Optional: e.g., "tar.gz", "zip". If empty, try to infer from name.
	StripComponents          int    `json:"stripComponents,omitempty"`          // Number of leading components to strip
	Overwrite                bool   `json:"overwrite,omitempty"`                // If true, overwrite if destination files/dir exist
	RemoveExtractedOnRollback bool  `json:"removeExtractedOnRollback,omitempty"` // If true, rollback will attempt to remove ExtractionDir.
	ExtractedDirSharedDataKey string `json:"extractedDirSharedDataKey,omitempty"`// Optional: Cache key to store the path of extraction.
}

// NewExtractArchiveStepSpec creates a new ExtractArchiveStepSpec.
func NewExtractArchiveStepSpec(
	name, description,
	archivePath, archivePathCacheKey, extractionDir, archiveType string,
	stripComponents int, overwrite bool, removeOnRollback bool,
	extractedDirKey string,
) *ExtractArchiveStepSpec {
	finalName := name
	if finalName == "" {
		src := archivePath
		if src == "" && archivePathCacheKey != "" {
			src = fmt.Sprintf("cacheKey:%s", archivePathCacheKey)
		}
		finalName = fmt.Sprintf("Extract %s", filepath.Base(src))
	}
	finalDescription := description
	if finalDescription == "" {
		srcText := archivePath
		if srcText == "" && archivePathCacheKey != "" {
			srcText = fmt.Sprintf("archive from cache key '%s'", archivePathCacheKey)
		} else {
			srcText = fmt.Sprintf("archive %s", srcText)
		}
		finalDescription = fmt.Sprintf("Extracts %s to %s", srcText, extractionDir)
		if stripComponents > 0 {
			finalDescription += fmt.Sprintf(" stripping %d component(s)", stripComponents)
		}
	}

	return &ExtractArchiveStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ArchivePath:              archivePath,
		ArchivePathCacheKey:      archivePathCacheKey,
		ExtractionDir:            extractionDir,
		ArchiveType:              archiveType,
		StripComponents:          stripComponents,
		Overwrite:                overwrite,
		RemoveExtractedOnRollback: removeOnRollback,
		ExtractedDirSharedDataKey: extractedDirKey,
	}
}

// Name returns the step's name.
func (s *ExtractArchiveStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ExtractArchiveStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ExtractArchiveStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ExtractArchiveStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ExtractArchiveStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ExtractArchiveStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ExtractArchiveStepSpec) getEffectiveArchivePath(ctx runtime.StepContext) (string, error) {
	if s.ArchivePath != "" {
		return s.ArchivePath, nil
	}
	if s.ArchivePathCacheKey != "" {
		// Try StepCache, then TaskCache, then ModuleCache
		val, found := ctx.StepCache().Get(s.ArchivePathCacheKey)
		if found {
			if pathStr, ok := val.(string); ok { return pathStr, nil }
			return "", fmt.Errorf("cached archive path (StepCache key %s) is not a string", s.ArchivePathCacheKey)
		}
		val, found = ctx.TaskCache().Get(s.ArchivePathCacheKey)
		if found {
			if pathStr, ok := val.(string); ok { return pathStr, nil }
			return "", fmt.Errorf("cached archive path (TaskCache key %s) is not a string", s.ArchivePathCacheKey)
		}
		val, found = ctx.ModuleCache().Get(s.ArchivePathCacheKey)
		if found {
			if pathStr, ok := val.(string); ok { return pathStr, nil }
			return "", fmt.Errorf("cached archive path (ModuleCache key %s) is not a string", s.ArchivePathCacheKey)
		}
		return "", fmt.Errorf("archive path cache key %s not found in any cache", s.ArchivePathCacheKey)
	}
	return "", fmt.Errorf("archivePath or archivePathCacheKey must be specified")
}


func (s *ExtractArchiveStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.ExtractionDir == "" {
		return false, fmt.Errorf("ExtractionDir not set for step %s on host %s", s.GetName(), host.GetName())
	}
	// ExtractedDirSharedDataKey is optional for output, so not checking for its presence here.

	if s.Overwrite { // If overwrite is set, precheck should indicate not done, to force re-extraction.
	    logger.Debug("Overwrite is true, extraction will proceed.")
	    return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if extraction directory exists and is not empty. This is a basic check.
	// A more robust check might involve looking for a specific file known to be in the archive.
	exists, err := conn.Exists(ctx.GoContext(), s.ExtractionDir)
	if err != nil {
		logger.Warn("Failed to check existence of extraction directory, will attempt extraction.", "dir", s.ExtractionDir, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Extraction directory does not exist. Extraction needed.", "dir", s.ExtractionDir)
		return false, nil
	}

	// Check if directory is empty. `ls -A` lists all entries except . and ..
	// If it outputs anything, directory is not empty.
	lsCmd := fmt.Sprintf("ls -A %s", s.ExtractionDir)
	stdout, _, lsErr := conn.Exec(ctx.GoContext(), lsCmd, &connector.ExecOptions{})
	if lsErr != nil {
		// This might happen if directory exists but is not readable, or ls is not available.
		logger.Warn("Failed to list contents of extraction directory, assuming extraction needed.", "dir", s.ExtractionDir, "error", lsErr)
		return false, nil
	}

	if strings.TrimSpace(string(stdout)) != "" {
		logger.Info("Extraction directory exists and is not empty. Assuming already extracted.", "dir", s.ExtractionDir)
		// If a specific output key is expected, confirm it's in cache or a known file exists.
		if s.ExtractedDirSharedDataKey != "" {
		    // For simplicity, if dir is not empty, we assume the key would have been set.
		    // A more robust check would be to see if a specific file/subdir exists that this key might point to.
		}
		return true, nil
	}

	logger.Info("Extraction directory exists but is empty. Extraction needed.", "dir", s.ExtractionDir)
	return false, nil
}

func (s *ExtractArchiveStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	archivePath, err := s.getEffectiveArchivePath(ctx)
	if err != nil {
		return fmt.Errorf("failed to determine archive path for step %s on host %s: %w", s.GetName(), host.GetName(), err)
	}
	if s.ExtractionDir == "" {
		return fmt.Errorf("ExtractionDir must be specified for step %s on host %s", s.GetName(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Determine sudo requirement for extraction directory
	sudo := utils.PathRequiresSudo(s.ExtractionDir)
	execOpts := &connector.ExecOptions{Sudo: sudo}

	// Ensure extraction directory exists
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.ExtractionDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create extraction directory %s (stderr: %s) on host %s: %w", s.ExtractionDir, string(stderrMkdir), host.GetName(), errMkdir)
	}

	if s.Overwrite {
	    logger.Debug("Overwrite is true, attempting to clean extraction directory before extracting.", "dir", s.ExtractionDir)
	    // This is a simple clean. A more robust way might be needed if dir contains unrelated files.
	    // rm -rf contents of directory: rm -rf /path/to/dir/* /path/to/dir/.* (excluding . and ..)
	    // For simplicity, if Overwrite means "ensure dir is clean for new content", just removing and recreating might be easier if safe.
	    // Or, tar/unzip often have overwrite flags.
	    // For now, assume tar/unzip will overwrite files they extract. If full clean is needed, rm -rf would be here.
	}


	// Determine archive type if not specified
	archiveType := s.ArchiveType
	if archiveType == "" {
		if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
			archiveType = "tar.gz"
		} else if strings.HasSuffix(archivePath, ".tar.bz2") || strings.HasSuffix(archivePath, ".tbz2") {
			archiveType = "tar.bz2"
		} else if strings.HasSuffix(archivePath, ".tar.xz") || strings.HasSuffix(archivePath, ".txz") {
			archiveType = "tar.xz"
		} else if strings.HasSuffix(archivePath, ".tar") {
			archiveType = "tar"
		} else if strings.HasSuffix(archivePath, ".zip") {
			archiveType = "zip"
		} else {
			return fmt.Errorf("unknown archive type for %s, please specify ArchiveType", archivePath)
		}
	}
	logger.Info("Extracting archive.", "path", archivePath, "type", archiveType, "destination", s.ExtractionDir)

	var extractCmd string
	switch archiveType {
	case "tar.gz", "tgz", "tar.bz2", "tbz2", "tar.xz", "txz", "tar":
		stripOpt := ""
		if s.StripComponents > 0 {
			stripOpt = fmt.Sprintf("--strip-components=%d", s.StripComponents)
		}
		// Use -C for directory, ensure archivePath is absolute or relative to a known CWD for the command.
		// Assuming archivePath is accessible. Tar typically needs read access to archive and write to -C dir.
		extractCmd = fmt.Sprintf("tar -xf %s -C %s %s", archivePath, s.ExtractionDir, stripOpt)
	case "zip":
		// Unzip doesn't have --strip-components directly. Overwrite is often -o.
		// Unzipping to a specific directory is done with -d.
		// For zip, if stripping is needed, it's usually done by extracting to a temp dir, then moving.
		// This example keeps it simple; if complex zip ops are needed, this step would need more logic.
		if s.StripComponents > 0 {
			logger.Warn("StripComponents is not directly supported for zip archives by this basic step. Files will be extracted with full paths.", "archive", archivePath)
		}
		overwriteOpt := ""
		if s.Overwrite { overwriteOpt = "-o" } // -o for overwrite
		extractCmd = fmt.Sprintf("unzip %s %s -d %s", overwriteOpt, archivePath, s.ExtractionDir)
	default:
		return fmt.Errorf("unsupported archive type: %s", archiveType)
	}

	_, stderrExtract, errExtract := conn.Exec(ctx.GoContext(), extractCmd, execOpts) // Sudo applied to extraction dir creation, maybe to extraction too.
	if errExtract != nil {
		return fmt.Errorf("failed to extract archive %s to %s (stderr: %s): %w", archivePath, s.ExtractionDir, string(stderrExtract), errExtract)
	}

	if s.ExtractedDirSharedDataKey != "" {
		// This usually stores s.ExtractionDir, or a specific sub-path if known.
		// For this generic step, s.ExtractionDir is the most reliable path to store.
		ctx.StepCache().Set(s.ExtractedDirSharedDataKey, s.ExtractionDir)
		logger.Debug("Stored extraction directory path in cache.", "key", s.ExtractedDirSharedDataKey, "path", s.ExtractionDir)
	}

	logger.Info("Archive extracted successfully.", "destination", s.ExtractionDir)
	return nil
}

func (s *ExtractArchiveStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	if !s.RemoveExtractedOnRollback {
		logger.Info("Rollback requested, but RemoveExtractedOnRollback is false. No action taken.")
		return nil
	}
	if s.ExtractionDir == "" {
		logger.Warn("Cannot perform rollback: ExtractionDir is not set.")
		return nil
	}

	logger.Info("Attempting to remove extraction directory for rollback (best effort).", "dir", s.ExtractionDir)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	// Use Exec for rm -rf with potential sudo
	rmCmd := fmt.Sprintf("rm -rf %s", s.ExtractionDir)
	rmOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.ExtractionDir)}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, rmOpts)

	if errRm != nil {
		logger.Error("Failed to remove extraction directory during rollback.", "dir", s.ExtractionDir, "stderr", string(stderrRm), "error", errRm)
		// Do not return error for rollback as it's best-effort.
	} else {
		logger.Info("Extraction directory removed successfully.", "dir", s.ExtractionDir)
	}

	if s.ExtractedDirSharedDataKey != "" {
		ctx.StepCache().Delete(s.ExtractedDirSharedDataKey)
		logger.Debug("Removed extraction directory path from cache.", "key", s.ExtractedDirSharedDataKey)
	}
	return nil
}

// Ensure ExtractArchiveStep implements the step.Step interface.
var _ step.Step = (*ExtractArchiveStep)(nil)
