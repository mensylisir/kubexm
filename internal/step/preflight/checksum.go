package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/util"
	"github.com/pkg/errors"
)

type FileChecksum struct {
	Path     string
	Checksum string
	Algo     string
}

type VerifyChecksumsStep struct {
	step.Base
	Files []FileChecksum
}

type VerifyChecksumsStepBuilder struct {
	step.Builder[VerifyChecksumsStepBuilder, *VerifyChecksumsStep]
}

func NewVerifyChecksumsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *VerifyChecksumsStepBuilder {
	s := &VerifyChecksumsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Verify checksums of downloaded artifacts", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(VerifyChecksumsStepBuilder).Init(s)
}

func (b *VerifyChecksumsStepBuilder) WithFiles(files []FileChecksum) *VerifyChecksumsStepBuilder {
	b.Step.Files = files
	return b
}

func (s *VerifyChecksumsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyChecksumsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Precheck is tricky here as we don't want to compute checksums twice.
	// We will do the check in Run and rely on it being fast.
	// Returning false ensures Run is always executed if the step is part of a plan.
	return false, nil
}

func (s *VerifyChecksumsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if len(s.Files) == 0 {
		logger.Info("No files specified for checksum verification. Skipping.")
		result.MarkCompleted("No files specified for checksum verification")
		return result, nil
	}

	logger.Info("Starting checksum verification for specified files...")

	for _, file := range s.Files {
		log := logger.With("file", file.Path, "algorithm", file.Algo)
		log.Info("Verifying checksum...")

		actualChecksum, err := util.ComputeFileChecksum(file.Path, file.Algo)
		if err != nil {
			err = errors.Wrapf(err, "failed to compute checksum for file '%s'", file.Path)
			result.MarkFailed(err, "Failed to compute checksum")
			return result, err
		}

		if !strings.EqualFold(actualChecksum, file.Checksum) {
			err = fmt.Errorf("checksum mismatch for file '%s': expected '%s', got '%s'", file.Path, file.Checksum, actualChecksum)
			result.MarkFailed(err, "Checksum mismatch")
			return result, err
		}

		log.Info("Checksum verified successfully.")
	}

	logger.Info("All file checksums verified successfully.")
	result.MarkCompleted("All checksums verified")
	return result, nil
}

func (s *VerifyChecksumsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for VerifyChecksumsStep is a no-op.")
	return nil
}

var _ step.Step = (*VerifyChecksumsStep)(nil)
