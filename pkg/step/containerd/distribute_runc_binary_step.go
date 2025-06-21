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
	// RuncBinaryLocalPathCacheKey is the task cache key for the local path of the downloaded runc binary.
	RuncBinaryLocalPathCacheKey = "RuncBinaryLocalPath"
	// RuncBinaryRemotePathCacheKey is the task cache key for the remote path of the distributed runc binary.
	RuncBinaryRemotePathCacheKey = "RuncBinaryRemotePath"
)

// DistributeRuncBinaryStep uploads the runc binary to target nodes.
type DistributeRuncBinaryStep struct {
	meta                      spec.StepMeta
	LocalBinaryPathCacheKey   string
	RemoteTempDir             string
	RemoteBinaryName          string // Name of the binary file on the remote node (e.g., runc.amd64 or runc)
	OutputRemotePathCacheKey  string
	Sudo                      bool
}

// NewDistributeRuncBinaryStep creates a new DistributeRuncBinaryStep.
func NewDistributeRuncBinaryStep(instanceName, localPathCacheKey, remoteTempDir, remoteBinaryName, outputRemotePathKey string, sudo bool) step.Step {
	if localPathCacheKey == "" {
		localPathCacheKey = RuncBinaryLocalPathCacheKey
	}
	if remoteTempDir == "" {
		remoteTempDir = "/tmp/kubexm-binaries" // Different temp dir for clarity
	}
	if outputRemotePathKey == "" {
		outputRemotePathKey = RuncBinaryRemotePathCacheKey
	}
	name := instanceName
	if name == "" {
		name = "DistributeRuncBinary"
	}
	return &DistributeRuncBinaryStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Uploads the runc binary to target nodes.",
		},
		LocalBinaryPathCacheKey:  localPathCacheKey,
		RemoteTempDir:            remoteTempDir,
		RemoteBinaryName:         remoteBinaryName, // Must be set by the task
		OutputRemotePathCacheKey: outputRemotePathKey,
		Sudo:                     sudo,
	}
}

func (s *DistributeRuncBinaryStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DistributeRuncBinaryStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if s.RemoteBinaryName == "" {
		return false, fmt.Errorf("RemoteBinaryName is not set for step %s", s.meta.Name)
	}
	remoteBinaryPath := filepath.Join(s.RemoteTempDir, s.RemoteBinaryName)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, remoteBinaryPath)
	if err != nil {
		logger.Warn("Failed to check for existing remote runc binary, will attempt upload.", "path", remoteBinaryPath, "error", err)
		return false, nil
	}
	if exists {
		logger.Info("Runc binary already exists on remote host.", "path", remoteBinaryPath)
		ctx.TaskCache().Set(s.OutputRemotePathCacheKey, remoteBinaryPath)
		return true, nil
	}
	logger.Info("Runc binary does not exist on remote host.", "path", remoteBinaryPath)
	return false, nil
}

func (s *DistributeRuncBinaryStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	localPathValue, found := ctx.TaskCache().Get(s.LocalBinaryPathCacheKey)
	if !found {
		return fmt.Errorf("local runc binary path not found in task cache with key '%s'", s.LocalBinaryPathCacheKey)
	}
	localPath, ok := localPathValue.(string)
	if !ok || localPath == "" {
		return fmt.Errorf("invalid local runc binary path in task cache: got '%v'", localPathValue)
	}
	if s.RemoteBinaryName == "" {
        s.RemoteBinaryName = filepath.Base(localPath)
        logger.Info("RemoteBinaryName not set, derived from local path.", "name", s.RemoteBinaryName)
    }
	if s.RemoteBinaryName == "" {
         return fmt.Errorf("RemoteBinaryName is empty and could not be derived for step %s", s.meta.Name)
    }
	logger.Info("Retrieved local runc binary path from cache.", "path", localPath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	logger.Info("Ensuring remote temporary directory exists.", "path", s.RemoteTempDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.RemoteTempDir, "0750", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote temp directory %s: %w", s.RemoteTempDir, err)
	}
	remoteBinaryPath := filepath.Join(s.RemoteTempDir, s.RemoteBinaryName)
	logger.Info("Uploading runc binary.", "local", localPath, "remote", remoteBinaryPath)
	fileTransferOptions := &connector.FileTransferOptions{ Permissions: "0755", Sudo: s.Sudo } // Runc needs to be executable
	err = runnerSvc.UploadFile(ctx.GoContext(), localPath, remoteBinaryPath, fileTransferOptions, host)
	if err != nil {
		return fmt.Errorf("failed to upload runc binary from %s to %s:%s: %w", localPath, host.GetName(), remoteBinaryPath, err)
	}
	logger.Info("Runc binary uploaded successfully.", "remotePath", remoteBinaryPath)
	ctx.TaskCache().Set(s.OutputRemotePathCacheKey, remoteBinaryPath)
	return nil
}

func (s *DistributeRuncBinaryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	if s.RemoteBinaryName == "" {
		cachedRemotePathVal, found := ctx.TaskCache().Get(s.OutputRemotePathCacheKey)
		if !found {
			logger.Warn("Remote binary path not in cache and RemoteBinaryName not set for rollback.")
			return nil
		}
		cachedRemotePath, ok := cachedRemotePathVal.(string)
		if !ok || cachedRemotePath == "" {
			logger.Warn("Invalid remote binary path in cache for rollback.", "value", cachedRemotePathVal)
			return nil
		}
		s.RemoteBinaryName = filepath.Base(cachedRemotePath)
		if s.RemoteBinaryName == "." || s.RemoteBinaryName == "/" {
            logger.Warn("Could not reliably derive RemoteBinaryName from cached path for rollback.", "cachedPath", cachedRemotePath)
            return nil
        }
	}
	remoteBinaryPath := filepath.Join(s.RemoteTempDir, s.RemoteBinaryName)
	logger.Info("Attempting to remove remote runc binary for rollback.", "path", remoteBinaryPath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil
	}
	if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteBinaryPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove remote runc binary during rollback (best effort).", "path", remoteBinaryPath, "error", err)
	} else {
		logger.Info("Successfully removed remote runc binary (if it existed).", "path", remoteBinaryPath)
	}
	ctx.TaskCache().Delete(s.OutputRemotePathCacheKey)
	return nil
}

var _ step.Step = (*DistributeRuncBinaryStep)(nil)
