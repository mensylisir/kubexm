package docker

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	// CriDockerdExtractedDirCacheKey is where the path to the dir containing extracted cri-dockerd binary and systemd files will be stored.
	CriDockerdExtractedDirCacheKey = "CriDockerdExtractedDir"
)

// ExtractCriDockerdArchiveStep extracts the cri-dockerd archive on a target node.
type ExtractCriDockerdArchiveStep struct {
	meta                       spec.StepMeta
	RemoteArchivePathCacheKey  string
	TargetExtractBaseDir       string // Base directory where a version/arch specific subdir might be created for extraction. e.g., /tmp/kubexm-extracted/cri-dockerd
	ArchiveInternalSubDir      string // cri-dockerd archives often extract to a "cri-dockerd" subdirectory.
	OutputExtractedPathCacheKey string
	Sudo                       bool
	RemoveArchiveAfterExtract  bool
}

// NewExtractCriDockerdArchiveStep creates a new ExtractCriDockerdArchiveStep.
func NewExtractCriDockerdArchiveStep(instanceName, remotePathCacheKey, targetExtractBaseDir, archiveInternalSubDir, outputPathCacheKey string, sudo, removeArchive bool) step.Step {
	if remotePathCacheKey == "" {
		remotePathCacheKey = CriDockerdArchiveRemotePathCacheKey
	}
	if targetExtractBaseDir == "" {
		targetExtractBaseDir = "/tmp/kubexm-extracted/cri-dockerd"
	}
	if archiveInternalSubDir == "" {
		// cri-dockerd tars usually have a 'cri-dockerd' root dir inside them.
		// e.g., cri-dockerd-0.3.1.amd64.tgz extracts to cri-dockerd/cri-dockerd
		archiveInternalSubDir = "cri-dockerd"
	}
	if outputPathCacheKey == "" {
		outputPathCacheKey = CriDockerdExtractedDirCacheKey
	}
	name := instanceName
	if name == "" {
		name = "ExtractCriDockerdArchive"
	}

	return &ExtractCriDockerdArchiveStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Extracts the cri-dockerd archive on the target node.",
		},
		RemoteArchivePathCacheKey: remotePathCacheKey,
		TargetExtractBaseDir:      targetExtractBaseDir,
		ArchiveInternalSubDir:     archiveInternalSubDir,
		OutputExtractedPathCacheKey:outputPathCacheKey,
		Sudo:                      sudo,
		RemoveArchiveAfterExtract: removeArchive,
	}
}

func (s *ExtractCriDockerdArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ExtractCriDockerdArchiveStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	expectedContentDir := filepath.Join(s.TargetExtractBaseDir, s.ArchiveInternalSubDir)
	finalCriDockerdBinaryPath := filepath.Join(expectedContentDir, "cri-dockerd") // The binary itself

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, finalCriDockerdBinaryPath)
	if exists {
		logger.Info("Key cri-dockerd binary already exists in target extraction directory.", "path", finalCriDockerdBinaryPath)
		ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, expectedContentDir)
		return true, nil
	}
	logger.Info("cri-dockerd binary not found in target extraction directory. Extraction needed.", "expected_at", finalCriDockerdBinaryPath)
	return false, nil
}

func (s *ExtractCriDockerdArchiveStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	remoteArchivePathVal, found := ctx.TaskCache().Get(s.RemoteArchivePathCacheKey)
	if !found {
		return fmt.Errorf("remote cri-dockerd archive path not found in task cache with key '%s'", s.RemoteArchivePathCacheKey)
	}
	remoteArchivePath, ok := remoteArchivePathVal.(string)
	if !ok || remoteArchivePath == "" {
		return fmt.Errorf("invalid remote cri-dockerd archive path in task cache: got '%v'", remoteArchivePathVal)
	}
	logger.Info("Retrieved remote cri-dockerd archive path from cache.", "path", remoteArchivePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Ensuring target extraction base directory exists.", "path", s.TargetExtractBaseDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetExtractBaseDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target extraction base directory %s: %w", s.TargetExtractBaseDir, err)
	}

	logger.Info("Extracting cri-dockerd archive.", "source", remoteArchivePath, "destinationDir", s.TargetExtractBaseDir)
	if err := runnerSvc.Extract(ctx.GoContext(), conn, nil, remoteArchivePath, s.TargetExtractBaseDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract cri-dockerd archive %s to %s: %w", remoteArchivePath, s.TargetExtractBaseDir, err)
	}
	logger.Info("cri-dockerd archive extracted successfully.")

	finalContentDir := filepath.Join(s.TargetExtractBaseDir, s.ArchiveInternalSubDir)

	expectedCriDockerdBinary := filepath.Join(finalContentDir, "cri-dockerd")
	exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, expectedCriDockerdBinary)
	if !exists {
		logger.Error("Post-extraction check failed: cri-dockerd binary not found.", "expected_at", expectedCriDockerdBinary)
		lsOutput, _, lsErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, fmt.Sprintf("ls -lR %s", s.TargetExtractBaseDir), &connector.ExecOptions{Sudo: s.Sudo})
		if lsErr == nil {
			logger.Info("Contents of extraction base directory", "dir", s.TargetExtractBaseDir, "listing", string(lsOutput))
		}
		return fmt.Errorf("cri-dockerd binary not found in %s after extraction", expectedCriDockerdBinary)
	}
	logger.Info("cri-dockerd binary confirmed in extraction path.", "path", expectedCriDockerdBinary)

	if s.RemoveArchiveAfterExtract {
		logger.Info("Removing source archive after extraction.", "path", remoteArchivePath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil {
			logger.Warn("Failed to remove source archive (best effort).", "path", remoteArchivePath, "error", err)
		}
	}

	ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, finalContentDir)
	return nil
}

func (s *ExtractCriDockerdArchiveStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
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

var _ step.Step = (*ExtractCriDockerdArchiveStep)(nil)
