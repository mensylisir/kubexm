package common

import (
	"fmt"
	"path/filepath"
	"strings"
	// "time" // No longer directly used by this step's logic, timeouts handled by context/connector
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// "github.com/mensylisir/kubexm/pkg/spec" // No longer needed
)

// ExtractArchiveStep extracts an archive file on a target host.
type ExtractArchiveStep struct {
	ArchivePathSharedDataKey string // Key to retrieve archive path from Task Cache (Mandatory input)
	ExtractionDir             string // Directory to extract contents to (Mandatory)
	ExtractedDirSharedDataKey string // Task Cache key to store the path of the primary extracted content (Mandatory for output)
	ArchiveType               string // Optional: e.g., "tar.gz", "zip". If empty, runner.Extract infers.
	PreserveOriginalArchive   bool   // Optional: Hint for cleanup steps. Defaults to false.
	RemoveExtractedOnRollback bool   // If true, rollback will attempt to remove ExtractionDir.
}

// NewExtractArchiveStep creates a new ExtractArchiveStep.
func NewExtractArchiveStep(
	archivePathKey, extractionDir, extractedDirKey string,
	archiveType string, preserveOriginal bool, removeOnRollback bool,
) step.Step {
	return &ExtractArchiveStep{
		ArchivePathSharedDataKey: archivePathKey,
		ExtractionDir:             extractionDir,
		ExtractedDirSharedDataKey: extractedDirKey,
		ArchiveType:               archiveType,
		PreserveOriginalArchive:   preserveOriginal,
		RemoveExtractedOnRollback: removeOnRollback,
	}
}

func (s *ExtractArchiveStep) Name() string {
	return "Extract Archive"
}

func (s *ExtractArchiveStep) Description() string {
	return fmt.Sprintf("Extracts archive (from cache key '%s') to '%s' (output key '%s')",
		s.ArchivePathSharedDataKey, s.ExtractionDir, s.ExtractedDirSharedDataKey)
}

func (s *ExtractArchiveStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")

	if s.ExtractionDir == "" {
		return false, fmt.Errorf("ExtractionDir not set for step %s on host %s", s.Name(), host.GetName())
	}
	if s.ExtractedDirSharedDataKey == "" {
		return false, fmt.Errorf("ExtractedDirSharedDataKey not set for step %s on host %s", s.Name(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	// Check if the target path is already in cache and exists
	extractedPathVal, pathOk := ctx.TaskCache().Get(s.ExtractedDirSharedDataKey)
	if pathOk {
		extractedPath, okStr := extractedPathVal.(string)
		if okStr && extractedPath != "" {
			// Assuming connector.Exists(ctx, path) (bool, error)
			exists, errCheck := conn.Exists(ctx.GoContext(), extractedPath)
			if errCheck != nil {
				logger.Warn("Error checking existence of configured extracted path from cache", "path", extractedPath, "error", errCheck)
				return false, nil // Treat error as "not done"
			}
			if exists {
				logger.Info("Extracted content path found in Task Cache and exists on disk. Assuming already extracted.", "path", extractedPath)
				return true, nil
			}
			logger.Info("Path from Task Cache key does not exist. Needs extraction.", "path", extractedPath, "key", s.ExtractedDirSharedDataKey)
		}
	}

	// Fallback: Check if ExtractionDir exists and is not empty
	// Assuming connector.Exists(ctx, path) (bool, error)
	dirExists, errCheckDir := conn.Exists(ctx.GoContext(), s.ExtractionDir)
	if errCheckDir != nil {
		logger.Warn("Error checking existence of ExtractionDir", "path", s.ExtractionDir, "error", errCheckDir)
		return false, nil // Treat error as "not done"
	}
	if dirExists {
		// Assuming connector.List(ctx, path) ([]string, error) - returns list of basenames or full paths
		items, errList := conn.List(ctx.GoContext(), s.ExtractionDir)
		if errList != nil {
			logger.Warn("Failed to list contents of ExtractionDir, proceeding with extraction attempt.", "path", s.ExtractionDir, "error", errList)
			return false, nil
		}
		if len(items) > 0 {
			logger.Info("Target ExtractionDir exists and is not empty. Execute will run to ensure cache key is set.", "path", s.ExtractionDir)
			return false, nil // Let Execute run to populate cache
		}
	}

	logger.Debug("Extraction not confirmed by cache or existing non-empty directory.")
	return false, nil
}

func (s *ExtractArchiveStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")

	if s.ExtractionDir == "" {
		return fmt.Errorf("ExtractionDir not set for step %s on host %s", s.Name(), host.GetName())
	}
	if s.ArchivePathSharedDataKey == "" {
		return fmt.Errorf("ArchivePathSharedDataKey not set for step %s on host %s", s.Name(), host.GetName())
	}
	if s.ExtractedDirSharedDataKey == "" {
		return fmt.Errorf("ExtractedDirSharedDataKey not set for step %s on host %s", s.Name(), host.GetName())
	}

	archivePathVal, archiveOk := ctx.TaskCache().Get(s.ArchivePathSharedDataKey)
	if !archiveOk {
		return fmt.Errorf("archive path not found in Task Cache using key '%s' for step %s on host %s", s.ArchivePathSharedDataKey, s.Name(), host.GetName())
	}
	archivePath, okStr := archivePathVal.(string)
	if !okStr || archivePath == "" {
		return fmt.Errorf("invalid or empty archive path in Task Cache using key '%s' for step %s on host %s", s.ArchivePathSharedDataKey, s.Name(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	logger.Info("Ensuring extraction directory exists", "path", s.ExtractionDir)
	// Assuming connector.Mkdir(ctx, path, permissions string) error
	if errMkdir := conn.Mkdir(ctx.GoContext(), s.ExtractionDir, "0755"); errMkdir != nil {
		return fmt.Errorf("failed to create extraction directory %s for step %s on host %s: %w", s.ExtractionDir, s.Name(), host.GetName(), errMkdir)
	}

	logger.Info("Extracting archive", "archive", archivePath, "destination", s.ExtractionDir)
	extractOpts := connector.ExtractOptions{
		ArchiveType: s.ArchiveType,
	}
	// Assuming connector.Extract(ctx, archivePath, targetDir, options ExtractOptions) error
	if errExtract := conn.Extract(ctx.GoContext(), archivePath, s.ExtractionDir, extractOpts); errExtract != nil {
		return fmt.Errorf("failed to extract archive %s to %s for step %s on host %s: %w", archivePath, s.ExtractionDir, s.Name(), host.GetName(), errExtract)
	}
	logger.Info("Archive extracted successfully.")

	var determinedExtractedPath string
	// Assuming connector.List(ctx, path) ([]string, error) - returns list of basenames
	items, errList := conn.List(ctx.GoContext(), s.ExtractionDir)
	if errList != nil {
		logger.Warn("Failed to list contents of extraction directory. Using extraction directory itself as extracted path.", "path", s.ExtractionDir, "error", errList)
		determinedExtractedPath = s.ExtractionDir
	} else {
		nonHiddenItems := []string{}
		for _, item := range items {
			if !strings.HasPrefix(filepath.Base(item), ".") { // Ensure we use the basename for prefix check
				nonHiddenItems = append(nonHiddenItems, item)
			}
		}
		if len(nonHiddenItems) == 1 {
			// If conn.List returns basenames:
			determinedExtractedPath = filepath.Join(s.ExtractionDir, nonHiddenItems[0])
			// If conn.List returns full paths, then:
			// determinedExtractedPath = nonHiddenItems[0]
			// For now, assuming basenames as it's more common for a simple List.
			logger.Debug("One non-hidden item found in extraction dir. Setting as primary extracted path.", "path", determinedExtractedPath)
		} else {
			logger.Debug("Multiple or no non-hidden items found. Using extraction directory as primary extracted path.", "count", len(nonHiddenItems), "path", s.ExtractionDir)
			determinedExtractedPath = s.ExtractionDir
		}
	}

	ctx.TaskCache().Set(s.ExtractedDirSharedDataKey, determinedExtractedPath)
	logger.Info("Stored extracted path in Task Cache.", "key", s.ExtractedDirSharedDataKey, "path", determinedExtractedPath)
	return nil
}

func (s *ExtractArchiveStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")

	if !s.RemoveExtractedOnRollback {
		logger.Info("Rollback requested, but RemoveExtractedOnRollback is false. No action taken.")
		return nil
	}
	if s.ExtractionDir == "" {
		logger.Warn("Cannot perform rollback: ExtractionDir is not set.")
		return nil
	}

	logger.Info("Attempting to remove extracted content directory for rollback.", "path", s.ExtractionDir)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Name(), err)
	}

	removeOpts := connector.RemoveOptions{Recursive: true, IgnoreNotExist: true}
	// Assuming connector.Remove(ctx, path, options RemoveOptions) error
	if errRemove := conn.Remove(ctx.GoContext(), s.ExtractionDir, removeOpts); errRemove != nil {
		logger.Error("Failed to remove extraction directory during rollback.", "path", s.ExtractionDir, "error", errRemove)
		return fmt.Errorf("failed to remove directory %s during rollback for step %s on host %s: %w", s.ExtractionDir, s.Name(), host.GetName(), errRemove)
	}

	logger.Info("Successfully removed extraction directory for rollback.", "path", s.ExtractionDir)
	ctx.TaskCache().Delete(s.ExtractedDirSharedDataKey) // Also remove from cache
	logger.Debug("Removed extracted path key from Task Cache.", "key", s.ExtractedDirSharedDataKey)
	return nil
}

// Ensure ExtractArchiveStep implements the step.Step interface.
var _ step.Step = (*ExtractArchiveStep)(nil)
