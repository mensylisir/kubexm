package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// Constants for Task Cache keys related to extraction
const (
	// EtcdExtractedDirCacheKey is the key for the task cache to store the path to the directory
	// where the etcd archive's contents (specifically etcd and etcdctl) are located after extraction.
	// This path is on the target etcd node.
	EtcdExtractedDirCacheKey = "EtcdExtractedDir"
)

// ExtractEtcdBinaryStep extracts the etcd binary archive on a target node.
type ExtractEtcdBinaryStep struct {
	meta                       spec.StepMeta
	RemoteArchivePathCacheKey  string // Task cache key for the path of the etcd archive on the target node.
	TargetExtractDir           string // Directory on the target node where the archive should be extracted.
	ArchiveSubDir              string // Optional: subdirectory within the archive that contains the binaries (e.g., "etcd-v3.5.9-linux-amd64"). If empty, assumes binaries are at the root of archive.
	OutputExtractedPathCacheKey string // Task cache key to store the path to the directory containing etcd/etcdctl.
	Sudo                       bool   // Whether to use sudo for extraction operations (e.g., if TargetExtractDir is restricted).
	RemoveArchiveAfterExtract  bool   // Whether to remove the source archive after successful extraction.
}

// NewExtractEtcdBinaryStep creates a new ExtractEtcdBinaryStep.
func NewExtractEtcdBinaryStep(instanceName, remotePathCacheKey, targetExtractDir, archiveSubDir, outputPathCacheKey string, sudo, removeArchive bool) step.Step {
	if remotePathCacheKey == "" {
		remotePathCacheKey = EtcdArchiveRemotePathCacheKey // Default from distribute step
	}
	if targetExtractDir == "" {
		targetExtractDir = "/tmp/kubexm-extracted/etcd" // Default temporary extraction location
	}
	if outputPathCacheKey == "" {
		outputPathCacheKey = EtcdExtractedDirCacheKey
	}
	name := instanceName
	if name == "" {
		name = "ExtractEtcdBinaryArchive"
	}

	return &ExtractEtcdBinaryStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Extracts the etcd binary archive on the target node.",
		},
		RemoteArchivePathCacheKey: remotePathCacheKey,
		TargetExtractDir:          targetExtractDir,
		ArchiveSubDir:             archiveSubDir,
		OutputExtractedPathCacheKey:outputPathCacheKey,
		Sudo:                      sudo,
		RemoveArchiveAfterExtract: removeArchive,
	}
}

func (s *ExtractEtcdBinaryStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ExtractEtcdBinaryStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	finalEtcdBinaryPath := filepath.Join(s.TargetExtractDir, s.ArchiveSubDir, "etcd")
	finalEtcdctlBinaryPath := filepath.Join(s.TargetExtractDir, s.ArchiveSubDir, "etcdctl")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	etcdExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, finalEtcdBinaryPath)
	etcdctlExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, finalEtcdctlBinaryPath)

	if etcdExists && etcdctlExists {
		logger.Info("Etcd binaries (etcd, etcdctl) already exist in target extraction directory.", "dir", filepath.Join(s.TargetExtractDir, s.ArchiveSubDir))
		// Store the path to the directory containing the binaries in the cache
		ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, filepath.Join(s.TargetExtractDir, s.ArchiveSubDir))
		return true, nil
	}

	if etcdExists && !etcdctlExists {
		logger.Info("etcd exists but etcdctl does not. Re-extraction needed.", "etcd_path", finalEtcdBinaryPath)
	} else if !etcdExists && etcdctlExists {
		logger.Info("etcdctl exists but etcd does not. Re-extraction needed.", "etcdctl_path", finalEtcdctlBinaryPath)
	} else {
		logger.Info("Neither etcd nor etcdctl found in target extraction directory. Extraction needed.")
	}
	return false, nil
}

func (s *ExtractEtcdBinaryStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	remoteArchivePathVal, found := ctx.TaskCache().Get(s.RemoteArchivePathCacheKey)
	if !found {
		return fmt.Errorf("remote etcd archive path not found in task cache with key '%s'", s.RemoteArchivePathCacheKey)
	}
	remoteArchivePath, ok := remoteArchivePathVal.(string)
	if !ok || remoteArchivePath == "" {
		return fmt.Errorf("invalid remote etcd archive path in task cache: got '%v'", remoteArchivePathVal)
	}
	logger.Info("Retrieved remote etcd archive path from cache.", "path", remoteArchivePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure TargetExtractDir exists
	logger.Info("Ensuring target extraction directory exists.", "path", s.TargetExtractDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetExtractDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target extraction directory %s: %w", s.TargetExtractDir, err)
	}

	logger.Info("Extracting etcd archive.", "source", remoteArchivePath, "destinationDir", s.TargetExtractDir)
	// Runner's Extract function should handle tar.gz by default.
	// It needs source path on host, and destination directory on host.
	if err := runnerSvc.Extract(ctx.GoContext(), conn, nil /* facts not needed for basic extract */, remoteArchivePath, s.TargetExtractDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract etcd archive %s to %s: %w", remoteArchivePath, s.TargetExtractDir, err)
	}
	logger.Info("Etcd archive extracted successfully.")

	// The actual path containing etcd/etcdctl might be TargetExtractDir itself, or TargetExtractDir/ArchiveSubDir
	finalBinaryContainerDir := s.TargetExtractDir
	if s.ArchiveSubDir != "" {
		finalBinaryContainerDir = filepath.Join(s.TargetExtractDir, s.ArchiveSubDir)
	}

	// Verify that etcd and etcdctl exist in the final location
	etcdPath := filepath.Join(finalBinaryContainerDir, "etcd")
	etcdctlPath := filepath.Join(finalBinaryContainerDir, "etcdctl")
	etcdExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, etcdPath)
	etcdctlExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, etcdctlPath)

	if !etcdExists || !etcdctlExists {
		logger.Error("Post-extraction check failed: one or both binaries (etcd, etcdctl) not found.", "expectedDir", finalBinaryContainerDir)
		// List contents of TargetExtractDir for debugging
		lsOutput, _, lsErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, fmt.Sprintf("ls -lR %s", s.TargetExtractDir), &connector.ExecOptions{Sudo: s.Sudo})
		if lsErr == nil {
			logger.Info("Contents of extraction directory", "dir", s.TargetExtractDir, "listing", string(lsOutput))
		}
		return fmt.Errorf("etcd/etcdctl not found in %s after extraction", finalBinaryContainerDir)
	}
	logger.Info("Binaries etcd and etcdctl confirmed in extraction path.", "dir", finalBinaryContainerDir)


	if s.RemoveArchiveAfterExtract {
		logger.Info("Removing source archive after extraction.", "path", remoteArchivePath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil { // Sudo might be needed if archive was placed with sudo
			logger.Warn("Failed to remove source archive (best effort).", "path", remoteArchivePath, "error", err)
		}
	}

	ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, finalBinaryContainerDir)
	return nil
}

func (s *ExtractEtcdBinaryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	// Remove the entire TargetExtractDir as rollback
	logger.Info("Attempting to remove target extraction directory for rollback.", "path", s.TargetExtractDir)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.TargetExtractDir, s.Sudo); err != nil {
		logger.Warn("Failed to remove target extraction directory during rollback (best effort).", "path", s.TargetExtractDir, "error", err)
	} else {
		logger.Info("Successfully removed target extraction directory (if it existed).", "path", s.TargetExtractDir)
	}

	ctx.TaskCache().Delete(s.OutputExtractedPathCacheKey)
	return nil
}

var _ step.Step = (*ExtractEtcdBinaryStep)(nil)
