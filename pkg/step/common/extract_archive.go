package common

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// ExtractArchiveStep extracts an archive file on a target host.
type ExtractArchiveStep struct {
	meta                      spec.StepMeta
	ArchivePathSharedDataKey string // Key to retrieve archive path from Task Cache (Mandatory input)
	ExtractionDir             string // Directory to extract contents to (Mandatory)
	ExtractedDirSharedDataKey string // Task Cache key to store the path of the primary extracted content (Mandatory for output)
	ArchiveType               string // Optional: e.g., "tar.gz", "zip". If empty, runner.Extract infers.
	Sudo                      bool   // Whether to use sudo for extraction and directory creation.
	PreserveOriginalArchive   bool   // Optional: Hint for cleanup steps. Defaults to false.
	RemoveExtractedOnRollback bool   // If true, rollback will attempt to remove ExtractionDir.
}

// NewExtractArchiveStep creates a new ExtractArchiveStep.
func NewExtractArchiveStep(
	instanceName string,
	archivePathKey, extractionDir, extractedDirKey string,
	archiveType string, sudo bool, preserveOriginal bool, removeOnRollback bool,
) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = "ExtractArchive"
	}
	return &ExtractArchiveStep{
		meta: spec.StepMeta{
			Name: metaName,
			Description: fmt.Sprintf("Extracts archive (from cache key '%s') to '%s' (output key '%s')",
				archivePathKey, extractionDir, extractedDirKey),
		},
		ArchivePathSharedDataKey: archivePathKey,
		ExtractionDir:             extractionDir,
		ExtractedDirSharedDataKey: extractedDirKey,
		ArchiveType:               archiveType,
		Sudo:                      sudo,
		PreserveOriginalArchive:   preserveOriginal,
		RemoveExtractedOnRollback: removeOnRollback,
	}
}

// Meta returns the step's metadata.
func (s *ExtractArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ExtractArchiveStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	if s.ExtractionDir == "" {
		return false, fmt.Errorf("ExtractionDir not set for step %s on host %s", s.meta.Name, host.GetName())
	}
	if s.ExtractedDirSharedDataKey == "" {
		return false, fmt.Errorf("ExtractedDirSharedDataKey not set for step %s on host %s", s.meta.Name, host.GetName())
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	// Check if the target path is already in cache and exists
	extractedPathVal, pathOk := ctx.TaskCache().Get(s.ExtractedDirSharedDataKey)
	if pathOk {
		extractedPath, okStr := extractedPathVal.(string)
		if okStr && extractedPath != "" {
			exists, errCheck := runnerSvc.Exists(ctx.GoContext(), conn, extractedPath)
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
	dirExists, errCheckDir := runnerSvc.Exists(ctx.GoContext(), conn, s.ExtractionDir)
	if errCheckDir != nil {
		logger.Warn("Error checking existence of ExtractionDir", "path", s.ExtractionDir, "error", errCheckDir)
		return false, nil // Treat error as "not done"
	}
	if dirExists {
		// Simplification: If ExtractionDir exists, assume it might be correctly populated.
		// The original logic tried to list contents, which is complex without a runner.List.
		// For precheck, if dir exists, let Run execute to ensure cache is populated.
		// A more robust precheck might involve checking for a specific sentinel file within ExtractionDir.
		logger.Info("Target ExtractionDir exists. Execute will run to ensure cache key is set.", "path", s.ExtractionDir)
		return false, nil // Let Execute run to populate cache
	}

	logger.Debug("Extraction not confirmed by cache or existing non-empty directory.")
	return false, nil
}

func (s *ExtractArchiveStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.ExtractionDir == "" {
		return fmt.Errorf("ExtractionDir not set for step %s on host %s", s.meta.Name, host.GetName())
	}
	if s.ArchivePathSharedDataKey == "" {
		return fmt.Errorf("ArchivePathSharedDataKey not set for step %s on host %s", s.meta.Name, host.GetName())
	}
	if s.ExtractedDirSharedDataKey == "" {
		return fmt.Errorf("ExtractedDirSharedDataKey not set for step %s on host %s", s.meta.Name, host.GetName())
	}

	archivePathVal, archiveOk := ctx.TaskCache().Get(s.ArchivePathSharedDataKey)
	if !archiveOk {
		return fmt.Errorf("archive path not found in Task Cache using key '%s' for step %s on host %s", s.ArchivePathSharedDataKey, s.meta.Name, host.GetName())
	}
	archivePath, okStr := archivePathVal.(string)
	if !okStr || archivePath == "" {
		return fmt.Errorf("invalid or empty archive path in Task Cache using key '%s' for step %s on host %s", s.ArchivePathSharedDataKey, s.meta.Name, host.GetName())
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}
	facts, err := ctx.GetHostFacts(host) // Facts are needed for runner.Extract
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	logger.Info("Ensuring extraction directory exists", "path", s.ExtractionDir)
	if errMkdir := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.ExtractionDir, "0755", s.Sudo); errMkdir != nil {
		return fmt.Errorf("failed to create extraction directory %s for step %s on host %s: %w", s.ExtractionDir, s.meta.Name, host.GetName(), errMkdir)
	}

	logger.Info("Extracting archive", "archive", archivePath, "destination", s.ExtractionDir)
	// Runner's Extract method might infer type if s.ArchiveType is empty.
	// It also needs facts and sudo flag.
	if errExtract := runnerSvc.Extract(ctx.GoContext(), conn, facts, archivePath, s.ExtractionDir, s.Sudo); errExtract != nil {
		return fmt.Errorf("failed to extract archive %s to %s for step %s on host %s: %w", archivePath, s.ExtractionDir, s.meta.Name, host.GetName(), errExtract)
	}
	logger.Info("Archive extracted successfully.")

	// Simplified logic for determinedExtractedPath.
	// Assumes the primary content is directly within ExtractionDir or a single, non-hidden subdirectory.
	// A more robust method might be needed if archives have complex structures.
	// For now, we'll assume the main extracted content is the directory itself or a uniquely named subdir.
	// The runner.Extract function in 7-runner设计.md does not specify return of extracted sub-path.
	// So, we will use s.ExtractionDir as the primary path for the cache.
	// Users of this step might need to know the internal structure of the archive if it's complex.
	determinedExtractedPath := s.ExtractionDir
	// Alternative: If archives predictably create a single top-level directory:
	// archiveBase := strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath)) // Attempt to get "etcd-v3.5.0-linux-amd64" from "...tar.gz"
	// if !strings.HasSuffix(archiveBase, ".tar") { // for .tar.gz
	// 	archiveBase = strings.TrimSuffix(archiveBase, ".tar")
	// }
	// potentialPath := filepath.Join(s.ExtractionDir, archiveBase)
	// exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, potentialPath)
	// if exists {
	// 	isDir, _ := runnerSvc.IsDir(ctx.GoContext(), conn, potentialPath)
	// 	if isDir {
	// 		determinedExtractedPath = potentialPath
	// 		logger.Debug("Determined extracted path to be a subdirectory.", "path", determinedExtractedPath)
	// 	}
	// }


	ctx.TaskCache().Set(s.ExtractedDirSharedDataKey, determinedExtractedPath)
	logger.Info("Stored extracted path in Task Cache.", "key", s.ExtractedDirSharedDataKey, "path", determinedExtractedPath)
	return nil
}

func (s *ExtractArchiveStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	if !s.RemoveExtractedOnRollback {
		logger.Info("Rollback requested, but RemoveExtractedOnRollback is false. No action taken.")
		return nil
	}
	if s.ExtractionDir == "" {
		logger.Warn("Cannot perform rollback: ExtractionDir is not set.")
		return nil
	}

	logger.Info("Attempting to remove extracted content directory for rollback.", "path", s.ExtractionDir)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.meta.Name, err)
	}

	if errRemove := runnerSvc.Remove(ctx.GoContext(), conn, s.ExtractionDir, s.Sudo); errRemove != nil {
		logger.Error("Failed to remove extraction directory during rollback.", "path", s.ExtractionDir, "error", errRemove)
		// Don't fail rollback if removal fails, just log, as per general guidance unless critical
		// return fmt.Errorf("failed to remove directory %s during rollback for step %s on host %s: %w", s.ExtractionDir, s.Meta().Name, host.GetName(), errRemove)
	}

	logger.Info("Successfully removed extraction directory for rollback (or removal was skipped/failed non-critically).", "path", s.ExtractionDir)
	ctx.TaskCache().Delete(s.ExtractedDirSharedDataKey) // Also remove from cache
	logger.Debug("Removed extracted path key from Task Cache.", "key", s.ExtractedDirSharedDataKey)
	return nil
}

// Ensure ExtractArchiveStep implements the step.Step interface.
var _ step.Step = (*ExtractArchiveStep)(nil)
