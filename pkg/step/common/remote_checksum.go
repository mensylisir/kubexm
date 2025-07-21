package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

type RemoteFileChecksumStep struct {
	step.Base
	FilePath          string
	ExpectedChecksum  string
	ChecksumAlgorithm string
}

type RemoteFileChecksumStepBuilder struct {
	step.Builder[RemoteFileChecksumStepBuilder, *RemoteFileChecksumStep]
}

func NewRemoteFileChecksumStepBuilder(ctx runtime.ExecutionContext, instanceName, filePath string) *RemoteFileChecksumStepBuilder {
	s := &RemoteFileChecksumStep{
		FilePath: filePath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Verify remote file checksum [%s]", instanceName, filePath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Second
	return new(RemoteFileChecksumStepBuilder).Init(s)
}

func (b *RemoteFileChecksumStepBuilder) WithExpectedChecksum(expectedChecksum string) *RemoteFileChecksumStepBuilder {
	b.Step.ExpectedChecksum = expectedChecksum
	return b
}

func (b *RemoteFileChecksumStepBuilder) WithChecksumAlgorithm(checksumAlgorithm string) *RemoteFileChecksumStepBuilder {
	b.Step.ChecksumAlgorithm = checksumAlgorithm
	return b
}

func (s *RemoteFileChecksumStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoteFileChecksumStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.FilePath)
	if err != nil {
		logger.Warn("Failed to check for file existence on remote host. Will proceed to Run phase.", "path", s.FilePath, "error", err)
		return false, nil
	}

	if !exists {
		err := fmt.Errorf("file to be checked does not exist on remote host: %s", s.FilePath)
		logger.Error(err, "Precheck failed")
		return false, err
	}
	logger.Info("File exists on remote host, proceeding to checksum verification in Run phase.", "path", s.FilePath)
	return false, nil
}

func (s *RemoteFileChecksumStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if s.ExpectedChecksum == "" {
		logger.Info("No expected checksum provided. Step considered successful.", "file", s.FilePath)
		return nil
	}
	if s.ChecksumAlgorithm == "" {
		return fmt.Errorf("checksum algorithm cannot be empty when expected checksum is provided for file %s", s.FilePath)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Verifying checksum on remote host...", "path", s.FilePath, "algorithm", s.ChecksumAlgorithm)
	err = runnerSvc.VerifyChecksum(ctx.GoContext(), conn, s.FilePath, s.ExpectedChecksum, s.ChecksumAlgorithm, s.Sudo)
	if err != nil {
		return fmt.Errorf("remote checksum verification failed for %s: %w", s.FilePath, err)
	}

	logger.Info("Checksum verified successfully on remote host.", "file", s.FilePath)
	return nil
}

func (s *RemoteFileChecksumStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.GetLogger().Info("RemoteFileChecksumStep has no rollback action.", "step", s.Meta().Name, "host", ctx.GetHost().GetName())
	return nil
}

var _ step.Step = (*RemoteFileChecksumStep)(nil)
