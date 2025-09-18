package pre

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util"
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

func NewVerifyChecksumsStepBuilder(ctx runtime.Context, instanceName string) *VerifyChecksumsStepBuilder {
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

func (s *VerifyChecksumsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if len(s.Files) == 0 {
		logger.Info("No files specified for checksum verification. Skipping.")
		return nil
	}

	logger.Info("Starting checksum verification for specified files...")

	for _, file := range s.Files {
		log := logger.With("file", file.Path, "algorithm", file.Algo)
		log.Info("Verifying checksum...")

		actualChecksum, err := util.ComputeFileChecksum(file.Path, file.Algo)
		if err != nil {
			return errors.Wrapf(err, "failed to compute checksum for file '%s'", file.Path)
		}

		if !strings.EqualFold(actualChecksum, file.Checksum) {
			return fmt.Errorf("checksum mismatch for file '%s': expected '%s', got '%s'", file.Path, file.Checksum, actualChecksum)
		}

		log.Info("Checksum verified successfully.")
	}

	logger.Info("All file checksums verified successfully.")
	return nil
}

func (s *VerifyChecksumsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for VerifyChecksumsStep is a no-op.")
	return nil
}

var _ step.Step = (*VerifyChecksumsStep)(nil)
