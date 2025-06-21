package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	// EtcdArchiveLocalPathCacheKey is the key for the task cache to get the local path of the downloaded etcd archive.
	EtcdArchiveLocalPathCacheKey = "EtcdArchiveLocalPath"
	// EtcdArchiveRemotePathCacheKey is the key for the task cache to store the remote path of the distributed etcd archive.
	EtcdArchiveRemotePathCacheKey = "EtcdArchiveRemotePath"
)

// DistributeEtcdBinaryStep uploads the etcd binary archive to target nodes.
type DistributeEtcdBinaryStep struct {
	meta                      spec.StepMeta
	LocalArchivePathCacheKey  string // Task cache key for the source path of the etcd archive on the control node.
	RemoteTempDir             string // Temporary directory on target nodes to upload the archive to.
	RemoteArchiveName         string // Name of the archive file on the remote node.
	OutputRemotePathCacheKey  string // Task cache key to store the full path of the archive on the remote node.
	Sudo                      bool   // Whether to use sudo for mkdir and upload (if runner needs it for specific paths).
}

// NewDistributeEtcdBinaryStep creates a new DistributeEtcdBinaryStep.
func NewDistributeEtcdBinaryStep(instanceName, localPathCacheKey, remoteTempDir, remoteArchiveName, outputRemotePathKey string, sudo bool) step.Step {
	if localPathCacheKey == "" {
		localPathCacheKey = EtcdArchiveLocalPathCacheKey
	}
	if remoteTempDir == "" {
		remoteTempDir = "/tmp/kubexm-archives"
	}
	if outputRemotePathKey == "" {
		outputRemotePathKey = EtcdArchiveRemotePathCacheKey
	}
	name := instanceName
	if name == "" {
		name = "DistributeEtcdBinaryArchive"
	}
	return &DistributeEtcdBinaryStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Uploads the etcd binary archive to target etcd nodes.",
		},
		LocalArchivePathCacheKey: localPathCacheKey,
		RemoteTempDir:            remoteTempDir,
		RemoteArchiveName:        remoteArchiveName, // Must be set, e.g., "etcd.tar.gz" or from another cache key
		OutputRemotePathCacheKey: outputRemotePathKey,
		Sudo:                     sudo,
	}
}

func (s *DistributeEtcdBinaryStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DistributeEtcdBinaryStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
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
		logger.Warn("Failed to check for existing remote archive, will attempt upload.", "path", remoteArchivePath, "error", err)
		return false, nil
	}
	if exists {
		// TODO: Add checksum verification if local checksum is available via cache and remote checksum can be obtained.
		// This would require the local archive's checksum to be passed or cached.
		logger.Info("Etcd archive already exists on remote host.", "path", remoteArchivePath)
		ctx.TaskCache().Set(s.OutputRemotePathCacheKey, remoteArchivePath)
		return true, nil
	}
	logger.Info("Etcd archive does not exist on remote host.", "path", remoteArchivePath)
	return false, nil
}

func (s *DistributeEtcdBinaryStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	localPathValue, found := ctx.TaskCache().Get(s.LocalArchivePathCacheKey)
	if !found {
		return fmt.Errorf("local etcd archive path not found in task cache with key '%s'", s.LocalArchivePathCacheKey)
	}
	localPath, ok := localPathValue.(string)
	if !ok || localPath == "" {
		return fmt.Errorf("invalid local etcd archive path in task cache: got '%v'", localPathValue)
	}

	if s.RemoteArchiveName == "" {
        // Attempt to use the base name of the local path if remote name not specified
        s.RemoteArchiveName = filepath.Base(localPath)
        logger.Info("RemoteArchiveName not set, derived from local path.", "name", s.RemoteArchiveName)
    }
	if s.RemoteArchiveName == "" {
         return fmt.Errorf("RemoteArchiveName is empty and could not be derived for step %s", s.meta.Name)
    }


	logger.Info("Retrieved local etcd archive path from cache.", "path", localPath)

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
	logger.Info("Uploading etcd archive.", "local", localPath, "remote", remoteArchivePath)

	// The runner.UploadFile should handle copying from the control node's localPath
	// (where LocalConnector is used for source) to the target host's remoteArchivePath.
	// This step itself runs on the *target* host context, but UploadFile is special.
	// It needs a way to access the control node's file system.
	// This implies that the runner's UploadFile method needs to be aware of the control node
	// or the StepContext needs to provide a way to get a connector to the control node.
	// For now, assuming runner.UploadFile is smart enough, or this step should run on control-node
	// with target host passed as a parameter to UploadFile.
	// Given typical step execution, this step runs *on* the target host.
	// Thus, localPath must be accessible *from* the control node where the plan is orchestrated.
	// The runner.UploadFile should be: runner.UploadFile(ctx, controlNodeConnector, localPath, targetHostConnector, remoteArchivePath, options)
	// This step definition implies it's simpler: runner.UploadFile(ctx, currentHostConnector, localPathOnControlNode, remotePathOnCurrentHost)
	// This means the `localPath` is resolved on the machine running `kubexm`, and then uploaded.
	// This is a common pattern. The localPath is from the perspective of the `kubexm` process.

	// Permissions for the uploaded file.
	fileTransferOptions := &connector.FileTransferOptions{
		Permissions: "0640", // Restrictive permissions for the archive
		Sudo: s.Sudo, // Sudo for the final placement if RemoteTempDir is restricted. Usually /tmp is not.
	}

	err = runnerSvc.UploadFile(ctx.GoContext(), localPath, remoteArchivePath, fileTransferOptions, host)
	if err != nil {
		return fmt.Errorf("failed to upload etcd archive from %s to %s:%s: %w", localPath, host.GetName(), remoteArchivePath, err)
	}

	logger.Info("Etcd archive uploaded successfully.", "remotePath", remoteArchivePath)
	ctx.TaskCache().Set(s.OutputRemotePathCacheKey, remoteArchivePath)
	return nil
}

func (s *DistributeEtcdBinaryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	if s.RemoteArchiveName == "" {
		// If name wasn't set and couldn't be derived during Run, this might be empty.
		// Try to get it from cache if Run succeeded and set it.
		cachedRemotePathVal, found := ctx.TaskCache().Get(s.OutputRemotePathCacheKey)
		if !found {
			logger.Warn("Remote archive path not in cache and RemoteArchiveName not set, cannot determine path for rollback.")
			return nil
		}
		cachedRemotePath, ok := cachedRemotePathVal.(string)
		if !ok || cachedRemotePath == "" {
			logger.Warn("Invalid remote archive path in cache for rollback.", "value", cachedRemotePathVal)
			return nil
		}
		s.RemoteArchiveName = filepath.Base(cachedRemotePath) // Attempt to derive from cached path
		if s.RemoteArchiveName == "." || s.RemoteArchiveName == "/" { // Basic sanity check
			logger.Warn("Could not reliably derive RemoteArchiveName from cached path for rollback.", "cachedPath", cachedRemotePath)
			return nil
		}
	}


	remoteArchivePath := filepath.Join(s.RemoteTempDir, s.RemoteArchiveName)
	logger.Info("Attempting to remove remote etcd archive for rollback.", "path", remoteArchivePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil {
		logger.Warn("Failed to remove remote etcd archive during rollback (best effort).", "path", remoteArchivePath, "error", err)
	} else {
		logger.Info("Successfully removed remote etcd archive (if it existed).", "path", remoteArchivePath)
	}

	ctx.TaskCache().Delete(s.OutputRemotePathCacheKey)
	return nil
}

var _ step.Step = (*DistributeEtcdBinaryStep)(nil)
