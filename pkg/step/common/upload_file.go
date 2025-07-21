package common

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type UploadFileStep struct {
	step.Base
	LocalSrcPath    string
	RemoteDestPath  string
	Permissions     string
	AllowMissingSrc bool
}

type UploadFileStepBuilder struct {
	step.Builder[UploadFileStepBuilder, *UploadFileStep]
}

func NewUploadFileStepBuilder(ctx runtime.ExecutionContext, instanceName, localSrc, remoteDest string) *UploadFileStepBuilder {
	cs := &UploadFileStep{
		LocalSrcPath:   localSrc,
		RemoteDestPath: remoteDest,
		Permissions:    "0644",
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Upload file from %s to %s", instanceName, localSrc, remoteDest)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 5 * time.Minute
	return new(UploadFileStepBuilder).Init(cs)
}

func (b *UploadFileStepBuilder) WithPermissions(permissions string) *UploadFileStepBuilder {
	b.Step.Permissions = permissions
	return b
}

func (b *UploadFileStepBuilder) WithAllowMissingSrc(allow bool) *UploadFileStepBuilder {
	b.Step.AllowMissingSrc = allow
	return b
}

func (s *UploadFileStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UploadFileStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localChecksum, err := calculateLocalFileChecksum(s.LocalSrcPath)
	if err != nil {
		if os.IsNotExist(err) {
			if s.AllowMissingSrc {
				logger.Infof("Local source file does not exist, but AllowMissingSrc is true. Step will be skipped. Path: %s", s.LocalSrcPath)
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to process local source file %s for precheck: %w", s.LocalSrcPath, err)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	remoteChecksumCmd := fmt.Sprintf("sha256sum %s | awk '{print $1}'", s.RemoteDestPath)
	output, runErr := runnerSvc.Run(ctx.GoContext(), conn, remoteChecksumCmd, s.Sudo)

	if runErr != nil {
		logger.Infof("Failed to get remote file checksum (file likely does not exist). Step needs to run. Path: %s", s.RemoteDestPath)
		return false, nil
	}
	remoteChecksum := string(output)

	if localChecksum == remoteChecksum {
		logger.Infof("Remote file exists and checksum matches. Step considered done. Path: %s", s.RemoteDestPath)
		return true, nil
	}

	logger.Infof("Remote file checksum does not match local file. Step needs to run. Local: %s, Remote: %s", localChecksum, remoteChecksum)
	return false, nil
}

func (s *UploadFileStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	content, err := os.ReadFile(s.LocalSrcPath)
	if err != nil {
		if os.IsNotExist(err) {
			if s.AllowMissingSrc {
				logger.Infof("Local source file does not exist, but AllowMissingSrc is true. Skipping upload. Path: %s", s.LocalSrcPath)
				return nil
			}
		}
		return fmt.Errorf("run: failed to read local source file %s: %w", s.LocalSrcPath, err)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Uploading file from %s to %s on host %s...", s.LocalSrcPath, s.RemoteDestPath, ctx.GetHost().GetName())
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, content, s.RemoteDestPath, s.Permissions, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to upload file to %s on host %s: %w", s.RemoteDestPath, ctx.GetHost().GetName(), err)
	}
	logger.Info("File uploaded successfully.")
	return nil
}

func (s *UploadFileStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Infof("Attempting rollback: removing remote file: %s", s.RemoteDestPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("rollback: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteDestPath, s.Sudo, false); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("Remote file was not present for rollback. Path: %s", s.RemoteDestPath)
			return nil
		}
		logger.Warnf("Failed to remove remote file during rollback (best effort). Path: %s, Error: %v", s.RemoteDestPath, err)
	} else {
		logger.Info("Remote file removed successfully during rollback.")
	}
	return nil
}

func calculateLocalFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

var _ step.Step = (*UploadFileStep)(nil)
