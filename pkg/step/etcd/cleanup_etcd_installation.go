package etcd

import (
	"fmt"

	"github.com/octo-cli/core/pkg/runtime"
	"github.com/octo-cli/core/pkg/step"
	"github.com/octo-cli/core/pkg/step/spec"
)

// CleanupEtcdInstallationStepSpec defines the specification for cleaning up etcd installation artifacts.
// No specific fields are needed as it relies on SharedData.
type CleanupEtcdInstallationStepSpec struct{}

// GetName returns the name of the step.
func (s *CleanupEtcdInstallationStepSpec) GetName() string {
	return "CleanupEtcdInstallation"
}

// CleanupEtcdInstallationStepExecutor implements the logic for cleaning up etcd installation artifacts.
type CleanupEtcdInstallationStepExecutor struct{}

// Check determines if cleanup is needed.
// It could check if the paths exist, or simply always return isDone = false to ensure it runs if included.
// For simplicity, we'll always attempt to run cleanup if the step is included.
// A more sophisticated check might verify if the paths in SharedData actually exist on disk.
func (e *CleanupEtcdInstallationStepExecutor) Check(ctx *runtime.Context, s spec.StepSpec) (bool, error) {
	// Check if paths exist in SharedData. If not, maybe previous steps failed or were skipped.
	archivePathVal, archiveOk := ctx.SharedData.Get(EtcdArchivePathKey)
	extractionDirVal, extractionOk := ctx.SharedData.Get(EtcdExtractionDirKey)

	if !archiveOk && !extractionOk {
		ctx.Logger.Infof("No etcd archive path or extraction directory found in SharedData. Cleanup might be unnecessary or previous steps failed.")
		// Nothing to clean based on SharedData, so consider it "done" in terms of this step's responsibility.
		return true, nil
	}

	// If paths are present, check if they actually exist on disk.
	// This is a more robust check.
	pathsToClean := []string{}
	if archiveOk {
		if archivePath, ok := archivePathVal.(string); ok && archivePath != "" {
			pathsToClean = append(pathsToClean, archivePath)
		}
	}
	if extractionOk {
		if extractionDir, ok := extractionDirVal.(string); ok && extractionDir != "" {
			pathsToClean = append(pathsToClean, extractionDir)
		}
	}

	for _, p := range pathsToClean {
		if _, err := ctx.Host.Runner.Stat(p); err == nil {
			ctx.Logger.Infof("Artifact %s exists and needs cleanup.", p)
			return false, nil // Found something to clean
		}
	}

	ctx.Logger.Infof("Etcd installation artifacts seem to be already cleaned up.")
	return true, nil // Nothing found on disk that matches SharedData paths.
}

// Execute removes the downloaded etcd archive and the extraction directory.
func (e *CleanupEtcdInstallationStepExecutor) Execute(ctx *runtime.Context, s spec.StepSpec) error {
	// _, ok := s.(*CleanupEtcdInstallationStepSpec)
	// if !ok {
	// 	return fmt.Errorf("invalid spec type %T for CleanupEtcdInstallationStepExecutor", s)
	// }

	ctx.Logger.Infof("Starting cleanup of etcd installation artifacts.")

	archivePathVal, archiveOk := ctx.SharedData.Get(EtcdArchivePathKey)
	if archiveOk {
		archivePath, ok := archivePathVal.(string)
		if ok && archivePath != "" {
			ctx.Logger.Infof("Removing etcd archive: %s", archivePath)
			if err := ctx.Host.Runner.Remove(archivePath); err != nil {
				// Log error but continue to attempt other cleanup tasks
				ctx.Logger.Errorf("Failed to remove etcd archive %s: %v", archivePath, err)
			} else {
				ctx.Logger.Infof("Successfully removed etcd archive: %s", archivePath)
				// Optionally, remove from SharedData after successful deletion
				// ctx.SharedData.Delete(EtcdArchivePathKey)
			}
		} else {
			ctx.Logger.Warnf("Invalid or empty etcd archive path in SharedData. Skipping removal.")
		}
	} else {
		ctx.Logger.Infof("No etcd archive path found in SharedData. Skipping archive removal.")
	}

	extractionDirVal, extractionOk := ctx.SharedData.Get(EtcdExtractionDirKey)
	if extractionOk {
		extractionDir, ok := extractionDirVal.(string)
		if ok && extractionDir != "" {
			ctx.Logger.Infof("Removing etcd extraction directory: %s", extractionDir)
			// Use RemoveAll for directories
			if err := ctx.Host.Runner.RemoveAll(extractionDir); err != nil {
				ctx.Logger.Errorf("Failed to remove etcd extraction directory %s: %v", extractionDir, err)
			} else {
				ctx.Logger.Infof("Successfully removed etcd extraction directory: %s", extractionDir)
				// Optionally, remove from SharedData
				// ctx.SharedData.Delete(EtcdExtractionDirKey)
				// ctx.SharedData.Delete(EtcdExtractedPathKey) // This was inside extractionDir
			}
		} else {
			ctx.Logger.Warnf("Invalid or empty etcd extraction directory path in SharedData. Skipping removal.")
		}
	} else {
		ctx.Logger.Infof("No etcd extraction directory path found in SharedData. Skipping directory removal.")
	}

	ctx.Logger.Infof("Etcd installation cleanup finished.")
	return nil // Errors are logged but don't necessarily stop the overall process if non-critical
}

func init() {
	step.Register(&CleanupEtcdInstallationStepSpec{}, &CleanupEtcdInstallationStepExecutor{})
}
