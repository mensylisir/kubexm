package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"path/filepath"
	"time"
)

type RemoteDownloadFileStep struct {
	step.Base
	URL          string
	DestPath     string
	Checksum     string
	ChecksumType string
}

type RemoteDownloadFileStepBuilder struct {
	step.Builder[RemoteDownloadFileStepBuilder, *RemoteDownloadFileStep]
}

func NewRemoteDownloadFileStepBuilder(instanceName, url, destPath string) *RemoteDownloadFileStepBuilder {
	s := &RemoteDownloadFileStep{
		URL:      url,
		DestPath: destPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download %s to remote path [%s]", instanceName, url, destPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Second
	return new(RemoteDownloadFileStepBuilder).Init(s)
}

func (b *RemoteDownloadFileStepBuilder) WithChecksum(checksum, checksumType string) *RemoteDownloadFileStepBuilder {
	b.Step.Checksum = checksum
	b.Step.ChecksumType = checksumType
	return b
}

func (s *RemoteDownloadFileStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoteDownloadFileStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.DestPath)
	if err != nil {
		logger.Warn("Failed to check for existing file on remote host, will attempt download.", "path", s.DestPath, "error", err)
		return false, nil
	}

	if !exists {
		logger.Info("Destination file does not exist on remote host, download required.", "path", s.DestPath)
		return false, nil
	}

	if s.Checksum == "" {
		logger.Info("Destination file exists on remote host and no checksum provided. Skipping.", "path", s.DestPath)
		return true, nil
	}

	logger.Info("Destination file exists, verifying checksum on remote host...", "path", s.DestPath)
	err = runnerSvc.VerifyChecksum(ctx.GoContext(), conn, s.DestPath, s.Checksum, s.ChecksumType, s.Sudo)
	if err == nil {
		logger.Info("Existing remote file is valid. Download will be skipped.", "path", s.DestPath)
		return true, nil
	}

	logger.Warn("Existing remote file checksum verification failed, will re-download.", "path", s.DestPath, "error", err)

	if removeErr := runnerSvc.Remove(ctx.GoContext(), conn, s.DestPath, s.Sudo, false); removeErr != nil {
		logger.Error(removeErr, "Failed to remove existing remote file with bad checksum.", "path", s.DestPath)
	}

	return false, nil
}

func (s *RemoteDownloadFileStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(s.DestPath)
	logger.Info("Ensuring remote directory exists.", "path", remoteDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
	}
	logger.Info("Starting download on remote host.", "url", s.URL, "dest", s.DestPath)
	if err := runnerSvc.Download(ctx.GoContext(), conn, facts, s.URL, s.DestPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to download on remote host: %w", err)
	}

	if s.Checksum != "" {
		logger.Info("Verifying checksum of downloaded file on remote host...", "path", s.DestPath)
		if err := runnerSvc.VerifyChecksum(ctx.GoContext(), conn, s.DestPath, s.Checksum, s.ChecksumType, s.Sudo); err != nil {
			_ = runnerSvc.Remove(ctx.GoContext(), conn, s.DestPath, s.Sudo, false)
			return fmt.Errorf("checksum verification failed for downloaded file: %w", err)
		}
		logger.Info("Checksum verified successfully.", "path", s.DestPath)
	}
	if outputKeyVal, ok := ctx.GetFromRuntimeConfig("outputCacheKey"); ok {
		if outputKey, isString := outputKeyVal.(string); isString && outputKey != "" {
			ctx.GetTaskCache().Set(outputKey, s.DestPath)
			logger.Info("Stored remote downloaded path in cache.", "key", outputKey, "path", s.DestPath)
		} else {
			return fmt.Errorf("invalid 'outputCacheKey' in RuntimeConfig: not a non-empty string")
		}
	}

	logger.Info("File downloaded successfully on remote host.", "path", s.DestPath)
	return nil
}

func (s *RemoteDownloadFileStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Attempting to remove downloaded file from remote host.", "path", s.DestPath)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.DestPath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove downloaded file during rollback.", "path", s.DestPath)
	}
	return nil
}

var _ step.Step = (*RemoteDownloadFileStep)(nil)
