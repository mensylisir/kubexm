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
	// CriDockerdArchiveLocalPathCacheKey is the task cache key for the local path of the downloaded cri-dockerd archive.
	CriDockerdArchiveLocalPathCacheKey = "CriDockerdArchiveLocalPath"
	// CriDockerdArchiveRemotePathCacheKey is the task cache key for the remote path of the distributed cri-dockerd archive.
	CriDockerdArchiveRemotePathCacheKey = "CriDockerdArchiveRemotePath"
)

// DistributeCriDockerdArchiveStep uploads the cri-dockerd archive to target nodes.
type DistributeCriDockerdArchiveStep struct {
	meta                      spec.StepMeta
	LocalArchivePathCacheKey  string
	RemoteTempDir             string
	RemoteArchiveName         string // Name of the archive file on the remote node (e.g., cri-dockerd.tgz)
	OutputRemotePathCacheKey  string
	Sudo                      bool
}

// NewDistributeCriDockerdArchiveStep creates a new DistributeCriDockerdArchiveStep.
func NewDistributeCriDockerdArchiveStep(instanceName, localPathCacheKey, remoteTempDir, remoteArchiveName, outputRemotePathKey string, sudo bool) step.Step {
	if localPathCacheKey == "" {
		localPathCacheKey = CriDockerdArchiveLocalPathCacheKey
	}
	if remoteTempDir == "" {
		remoteTempDir = "/tmp/kubexm-archives"
	}
	if outputRemotePathKey == "" {
		outputRemotePathKey = CriDockerdArchiveRemotePathCacheKey
	}
	name := instanceName
	if name == "" {
		name = "DistributeCriDockerdArchive"
	}
	return &DistributeCriDockerdArchiveStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Uploads the cri-dockerd archive to target nodes.",
		},
		LocalArchivePathCacheKey: localPathCacheKey,
		RemoteTempDir:            remoteTempDir,
		RemoteArchiveName:        remoteArchiveName,
		OutputRemotePathCacheKey: outputRemotePathKey,
		Sudo:                     sudo,
	}
}

func (s *DistributeCriDockerdArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DistributeCriDockerdArchiveStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if s.RemoteArchiveName == "" {
		return false, fmt.Errorf("RemoteArchiveName is not set for step %s", s.meta.Name)
	}
	remoteArchivePath := filepath.Join(s.RemoteTempDir, s.RemoteArchiveName)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, remoteArchivePath)
	if err != nil {
		logger.Warn("Failed to check for existing remote cri-dockerd archive, will attempt upload.", "path", remoteArchivePath, "error", err)
		return false, nil
	}
	if exists {
		logger.Info("cri-dockerd archive already exists on remote host.", "path", remoteArchivePath)
		ctx.TaskCache().Set(s.OutputRemotePathCacheKey, remoteArchivePath)
		return true, nil
	}
	logger.Info("cri-dockerd archive does not exist on remote host.", "path", remoteArchivePath)
	return false, nil
}

func (s *DistributeCriDockerdArchiveStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	localPathValue, found := ctx.TaskCache().Get(s.LocalArchivePathCacheKey)
	if !found {
		return fmt.Errorf("local cri-dockerd archive path not found in task cache with key '%s'", s.LocalArchivePathCacheKey)
	}
	localPath, ok := localPathValue.(string)
	if !ok || localPath == "" {
		return fmt.Errorf("invalid local cri-dockerd archive path in task cache: got '%v'", localPathValue)
	}
	if s.RemoteArchiveName == "" {
        s.RemoteArchiveName = filepath.Base(localPath)
        logger.Info("RemoteArchiveName not set, derived from local path.", "name", s.RemoteArchiveName)
    }
	if s.RemoteArchiveName == "" {
         return fmt.Errorf("RemoteArchiveName is empty and could not be derived for step %s", s.meta.Name)
    }
	logger.Info("Retrieved local cri-dockerd archive path from cache.", "path", localPath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	logger.Info("Ensuring remote temporary directory exists.", "path", s.RemoteTempDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.RemoteTempDir, "0750", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote temp directory %s: %w", s.RemoteTempDir, err)
	}
	remoteArchivePath := filepath.Join(s.RemoteTempDir, s.RemoteArchiveName)
	logger.Info("Uploading cri-dockerd archive.", "local", localPath, "remote", remoteArchivePath)
	fileTransferOptions := &connector.FileTransferOptions{ Permissions: "0640", Sudo: s.Sudo }
	err = runnerSvc.UploadFile(ctx.GoContext(), localPath, remoteArchivePath, fileTransferOptions, host)
	if err != nil {
		return fmt.Errorf("failed to upload cri-dockerd archive from %s to %s:%s: %w", localPath, host.GetName(), remoteArchivePath, err)
	}
	logger.Info("cri-dockerd archive uploaded successfully.", "remotePath", remoteArchivePath)
	ctx.TaskCache().Set(s.OutputRemotePathCacheKey, remoteArchivePath)
	return nil
}

func (s *DistributeCriDockerdArchiveStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	if s.RemoteArchiveName == "" {
		cachedRemotePathVal, found := ctx.TaskCache().Get(s.OutputRemotePathCacheKey)
		if !found {
			logger.Warn("Remote archive path not in cache and RemoteArchiveName not set for rollback.")
			return nil
		}
		cachedRemotePath, ok := cachedRemotePathVal.(string)
		if !ok || cachedRemotePath == "" {
			logger.Warn("Invalid remote archive path in cache for rollback.", "value", cachedRemotePathVal)
			return nil
		}
		s.RemoteArchiveName = filepath.Base(cachedRemotePath)
		if s.RemoteArchiveName == "." || s.RemoteArchiveName == "/" {
            logger.Warn("Could not reliably derive RemoteArchiveName from cached path for rollback.", "cachedPath", cachedRemotePath)
            return nil
        }
	}
	remoteArchivePath := filepath.Join(s.RemoteTempDir, s.RemoteArchiveName)
	logger.Info("Attempting to remove remote cri-dockerd archive for rollback.", "path", remoteArchivePath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil
	}
	if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil {
		logger.Warn("Failed to remove remote cri-dockerd archive during rollback (best effort).", "path", remoteArchivePath, "error", err)
	} else {
		logger.Info("Successfully removed remote cri-dockerd archive (if it existed).", "path", remoteArchivePath)
	}
	ctx.TaskCache().Delete(s.OutputRemotePathCacheKey)
	return nil
}

var _ step.Step = (*DistributeCriDockerdArchiveStep)(nil)
