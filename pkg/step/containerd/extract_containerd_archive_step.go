package containerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	// ContainerdExtractedDirCacheKey is where the path to the dir containing extracted binaries (like 'bin/') will be stored.
	ContainerdExtractedDirCacheKey = "ContainerdExtractedDir"
)

// ExtractContainerdArchiveStep extracts the containerd archive on a target node.
type ExtractContainerdArchiveStep struct {
	meta                       spec.StepMeta
	RemoteArchivePathCacheKey  string
	TargetExtractBaseDir       string // Base directory where a version/arch specific subdir will be created for extraction. e.g., /tmp/kubexm-extracted/containerd
	ArchiveInternalSubDir      string // Optional: if the .tar.gz extracts to e.g. "containerd-1.x.y", this is "containerd-1.x.y"
	OutputExtractedPathCacheKey string
	Sudo                       bool
	RemoveArchiveAfterExtract  bool
}

// NewExtractContainerdArchiveStep creates a new ExtractContainerdArchiveStep.
func NewExtractContainerdArchiveStep(instanceName, remotePathCacheKey, targetExtractBaseDir, archiveInternalSubDir, outputPathCacheKey string, sudo, removeArchive bool) step.Step {
	if remotePathCacheKey == "" {
		remotePathCacheKey = ContainerdArchiveRemotePathCacheKey
	}
	if targetExtractBaseDir == "" {
		targetExtractBaseDir = "/tmp/kubexm-extracted/containerd"
	}
	if outputPathCacheKey == "" {
		outputPathCacheKey = ContainerdExtractedDirCacheKey
	}
	name := instanceName
	if name == "" {
		name = "ExtractContainerdArchive"
	}

	return &ExtractContainerdArchiveStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Extracts the containerd archive on the target node.",
		},
		RemoteArchivePathCacheKey: remotePathCacheKey,
		TargetExtractBaseDir:      targetExtractBaseDir,
		ArchiveInternalSubDir:     archiveInternalSubDir, // Task should determine this, often empty if tar --strip-components=1 is used or archive is flat
		OutputExtractedPathCacheKey:outputPathCacheKey,
		Sudo:                      sudo,
		RemoveArchiveAfterExtract: removeArchive,
	}
}

func (s *ExtractContainerdArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ExtractContainerdArchiveStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	// The actual directory containing 'bin/' might be TargetExtractBaseDir or TargetExtractBaseDir/ArchiveInternalSubDir
	expectedBinDirContainer := s.TargetExtractBaseDir
	if s.ArchiveInternalSubDir != "" {
		expectedBinDirContainer = filepath.Join(s.TargetExtractBaseDir, s.ArchiveInternalSubDir)
	}
	// Check for a key binary, e.g., containerd itself, expected under a 'bin' subdirectory.
	finalContainerdBinaryPath := filepath.Join(expectedBinDirContainer, "bin", "containerd")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, finalContainerdBinaryPath)
	if exists {
		logger.Info("Key containerd binary already exists in target extraction directory.", "path", finalContainerdBinaryPath)
		ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, expectedBinDirContainer) // This path is the one containing 'bin/'
		return true, nil
	}
	logger.Info("Containerd binary not found in target extraction directory. Extraction needed.", "expected_at", finalContainerdBinaryPath)
	return false, nil
}

func (s *ExtractContainerdArchiveStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	remoteArchivePathVal, found := ctx.TaskCache().Get(s.RemoteArchivePathCacheKey)
	if !found {
		return fmt.Errorf("remote containerd archive path not found in task cache with key '%s'", s.RemoteArchivePathCacheKey)
	}
	remoteArchivePath, ok := remoteArchivePathVal.(string)
	if !ok || remoteArchivePath == "" {
		return fmt.Errorf("invalid remote containerd archive path in task cache: got '%v'", remoteArchivePathVal)
	}
	logger.Info("Retrieved remote containerd archive path from cache.", "path", remoteArchivePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Ensuring target extraction base directory exists.", "path", s.TargetExtractBaseDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetExtractBaseDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target extraction base directory %s: %w", s.TargetExtractBaseDir, err)
	}

	logger.Info("Extracting containerd archive.", "source", remoteArchivePath, "destinationDir", s.TargetExtractBaseDir)
	// Runner's Extract should handle .tar.gz. It might need options for --strip-components if ArchiveInternalSubDir is complex.
	// For now, assume it extracts into TargetExtractBaseDir, and if ArchiveInternalSubDir is set,
	// the actual content is inside TargetExtractBaseDir/ArchiveInternalSubDir.
	if err := runnerSvc.Extract(ctx.GoContext(), conn, nil, remoteArchivePath, s.TargetExtractBaseDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract containerd archive %s to %s: %w", remoteArchivePath, s.TargetExtractBaseDir, err)
	}
	logger.Info("Containerd archive extracted successfully.")

	finalContentDir := s.TargetExtractBaseDir
	if s.ArchiveInternalSubDir != "" {
		finalContentDir = filepath.Join(s.TargetExtractBaseDir, s.ArchiveInternalSubDir)
	}

	// Verify that a key binary (e.g., bin/containerd) exists in the final location
	expectedContainerdBinary := filepath.Join(finalContentDir, "bin", "containerd")
	exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, expectedContainerdBinary)
	if !exists {
		logger.Error("Post-extraction check failed: containerd binary not found.", "expected_at", expectedContainerdBinary)
		lsOutput, _, lsErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, fmt.Sprintf("ls -lR %s", s.TargetExtractBaseDir), &connector.ExecOptions{Sudo: s.Sudo})
		if lsErr == nil {
			logger.Info("Contents of extraction base directory", "dir", s.TargetExtractBaseDir, "listing", string(lsOutput))
		}
		return fmt.Errorf("containerd binary not found in %s after extraction", expectedContainerdBinary)
	}
	logger.Info("Containerd binary confirmed in extraction path.", "path", expectedContainerdBinary)

	if s.RemoveArchiveAfterExtract {
		logger.Info("Removing source archive after extraction.", "path", remoteArchivePath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil {
			logger.Warn("Failed to remove source archive (best effort).", "path", remoteArchivePath, "error", err)
		}
	}

	ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, finalContentDir) // This is the path to the dir containing 'bin', etc.
	return nil
}

func (s *ExtractContainerdArchiveStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove target extraction base directory for rollback.", "path", s.TargetExtractBaseDir)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil
	}
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.TargetExtractBaseDir, s.Sudo); err != nil {
		logger.Warn("Failed to remove target extraction base directory during rollback (best effort).", "path", s.TargetExtractBaseDir, "error", err)
	} else {
		logger.Info("Successfully removed target extraction base directory (if it existed).", "path", s.TargetExtractBaseDir)
	}
	ctx.TaskCache().Delete(s.OutputExtractedPathCacheKey)
	return nil
}

var _ step.Step = (*ExtractContainerdArchiveStep)(nil)
