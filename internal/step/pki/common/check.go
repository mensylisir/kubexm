package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CheckCertExpirationStep checks certificate expiration.
type CheckCertExpirationStep struct {
	step.Base
	CertPath    string
	WarningDays int
	OutputKey   string // Context key to store result
}

type CheckCertExpirationStepBuilder struct {
	step.Builder[CheckCertExpirationStepBuilder, *CheckCertExpirationStep]
}

func NewCheckCertExpirationStepBuilder(ctx runtime.ExecutionContext, instanceName, certPath, outputKey string) *CheckCertExpirationStepBuilder {
	s := &CheckCertExpirationStep{
		CertPath:    certPath,
		WarningDays: 30,
		OutputKey:   outputKey,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check certificate expiration for %s", instanceName, certPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CheckCertExpirationStepBuilder).Init(s)
}

func (b *CheckCertExpirationStepBuilder) WithWarningDays(days int) *CheckCertExpirationStepBuilder {
	b.Step.WarningDays = days
	return b
}

func (s *CheckCertExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckCertExpirationStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CheckCertExpirationStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("openssl x509 -in %s -noout -dates 2>/dev/null || echo 'invalid'", s.CertPath)
	logger.Infof("Running: %s", cmd)

	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	if err != nil {
		logger.Warnf("Failed to check cert: %v", err)
	}

	if s.OutputKey != "" {
		ctx.Export("task", s.OutputKey, runResult.Stdout)
	}

	result.MarkCompleted("Certificate expiration checked")
	return result, nil
}

func (s *CheckCertExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CheckCertExpirationStep)(nil)
