package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// VerifyChecksumStep verifies file checksum.
type VerifyChecksumStep struct {
	step.Base
	FilePath    string
	ExpectedSHA string
}

type VerifyChecksumStepBuilder struct {
	step.Builder[VerifyChecksumStepBuilder, *VerifyChecksumStep]
}

func NewVerifyChecksumStepBuilder(ctx runtime.ExecutionContext, instanceName, filePath, expectedSHA string) *VerifyChecksumStepBuilder {
	s := &VerifyChecksumStep{
		FilePath:    filePath,
		ExpectedSHA: expectedSHA,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Verify checksum of %s", instanceName, filePath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(VerifyChecksumStepBuilder).Init(s)
}

func (s *VerifyChecksumStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyChecksumStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *VerifyChecksumStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("sha256sum %s", s.FilePath)
	logger.Infof("Running: %s", cmd)

	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	if err != nil {
		result.MarkFailed(err, "failed to compute checksum")
		return result, err
	}

	// Parse result: "sha256sum filename" format
	var computedSHA string
	if len(runResult.Stdout) >= 64 {
		computedSHA = runResult.Stdout[:64]
	}

	if computedSHA != s.ExpectedSHA {
		result.MarkFailed(fmt.Errorf("checksum mismatch: expected %s, got %s", s.ExpectedSHA, computedSHA), "checksum mismatch")
		return result, fmt.Errorf("checksum mismatch")
	}

	logger.Infof("Checksum verified successfully")
	result.MarkCompleted("Checksum verified")
	return result, nil
}

func (s *VerifyChecksumStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*VerifyChecksumStep)(nil)
