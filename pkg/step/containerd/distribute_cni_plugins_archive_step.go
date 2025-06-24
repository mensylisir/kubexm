package containerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	// CNIPluginsArchiveLocalPathCacheKey is the task cache key for the local path of the downloaded CNI plugins archive.
	CNIPluginsArchiveLocalPathCacheKey = "CNIPluginsArchiveLocalPath"
	// CNIPluginsArchiveRemotePathCacheKey is the task cache key for the remote path of the distributed CNI plugins archive.
	CNIPluginsArchiveRemotePathCacheKey = "CNIPluginsArchiveRemotePath"
)

// DistributeCNIPluginsArchiveStep uploads the CNI plugins archive to target nodes.
type DistributeCNIPluginsArchiveStep struct {
	meta                     spec.StepMeta
	LocalArchivePathCacheKey string
	RemoteTempDir            string
	RemoteArchiveName        string // Name of the archive file on the remote node (e.g., cni-plugins.tgz)
	OutputRemotePathCacheKey string
	Sudo                     bool
}

// NewDistributeCNIPluginsArchiveStep creates a new DistributeCNIPluginsArchiveStep.
func NewDistributeCNIPluginsArchiveStep(instanceName, localPathCacheKey, remoteTempDir, remoteArchiveName, outputRemotePathKey string, sudo bool) step.Step {
	if localPathCacheKey == "" {
		localPathCacheKey = CNIPluginsArchiveLocalPathCacheKey
	}
	if remoteTempDir == "" {
		remoteTempDir = "/tmp/kubexm-archives"
	}
	if outputRemotePathKey == "" {
		outputRemotePathKey = CNIPluginsArchiveRemotePathCacheKey
	}
	name := instanceName
	if name == "" {
		name = "DistributeCNIPluginsArchive"
	}
	return &DistributeCNIPluginsArchiveStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Uploads the CNI plugins archive to target nodes.",
		},
		LocalArchivePathCacheKey: localPathCacheKey,
		RemoteTempDir:            remoteTempDir,
		RemoteArchiveName:        remoteArchiveName, // Must be set by the task
		OutputRemotePathCacheKey: outputRemotePathKey,
		Sudo:                     sudo,
	}
}

func (s *DistributeCNIPluginsArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DistributeCNIPluginsArchiveStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
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
		logger.Warn("Failed to check for existing remote CNI plugins archive, will attempt upload.", "path", remoteArchivePath, "error", err)
		return false, nil
	}
	if exists {
		logger.Info("CNI plugins archive already exists on remote host.", "path", remoteArchivePath)
		ctx.GetTaskCache().Set(s.OutputRemotePathCacheKey, remoteArchivePath)
		return true, nil
	}
	logger.Info("CNI plugins archive does not exist on remote host.", "path", remoteArchivePath)
	return false, nil
}

func (s *DistributeCNIPluginsArchiveStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	localPathValue, found := ctx.GetTaskCache().Get(s.LocalArchivePathCacheKey)
	if !found {
		return fmt.Errorf("local CNI plugins archive path not found in task cache with key '%s'", s.LocalArchivePathCacheKey)
	}
	localPath, ok := localPathValue.(string)
	if !ok || localPath == "" {
		return fmt.Errorf("invalid local CNI plugins archive path in task cache: got '%v'", localPathValue)
	}
	if s.RemoteArchiveName == "" {
		s.RemoteArchiveName = filepath.Base(localPath)
		logger.Info("RemoteArchiveName not set, derived from local path.", "name", s.RemoteArchiveName)
	}
	if s.RemoteArchiveName == "" {
		return fmt.Errorf("RemoteArchiveName is empty and could not be derived for step %s", s.meta.Name)
	}
	logger.Info("Retrieved local CNI plugins archive path from cache.", "path", localPath)
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
	logger.Info("Uploading CNI plugins archive.", "local", localPath, "remote", remoteArchivePath)
	fileTransferOptions := &connector.FileTransferOptions{Permissions: "0640", Sudo: s.Sudo}
	err = runnerSvc.UploadFile(ctx.GoContext(), localPath, remoteArchivePath, fileTransferOptions, host)
	if err != nil {
		return fmt.Errorf("failed to upload CNI plugins archive from %s to %s:%s: %w", localPath, host.GetName(), remoteArchivePath, err)
	}
	logger.Info("CNI plugins archive uploaded successfully.", "remotePath", remoteArchivePath)
	ctx.GetTaskCache().Set(s.OutputRemotePathCacheKey, remoteArchivePath)
	return nil
}

func (s *DistributeCNIPluginsArchiveStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	if s.RemoteArchiveName == "" {
		cachedRemotePathVal, found := ctx.GetTaskCache().Get(s.OutputRemotePathCacheKey)
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
	logger.Info("Attempting to remove remote CNI plugins archive for rollback.", "path", remoteArchivePath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil
	}
	if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil {
		logger.Warn("Failed to remove remote CNI plugins archive during rollback (best effort).", "path", remoteArchivePath, "error", err)
	} else {
		logger.Info("Successfully removed remote CNI plugins archive (if it existed).", "path", remoteArchivePath)
	}
	ctx.GetTaskCache().Delete(s.OutputRemotePathCacheKey)
	return nil
}

var _ step.Step = (*DistributeCNIPluginsArchiveStep)(nil)
