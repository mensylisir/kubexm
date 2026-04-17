package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyHarborCertStep copies Harbor certificates to remote host.
type CopyHarborCertStep struct {
	step.Base
	SourceCertPath string
	RemoteCertPath string
	Mode           string
}

type CopyHarborCertStepBuilder struct {
	step.Builder[CopyHarborCertStepBuilder, *CopyHarborCertStep]
}

func NewCopyHarborCertStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceCertPath, remoteCertPath, mode string) *CopyHarborCertStepBuilder {
	s := &CopyHarborCertStep{
		SourceCertPath: sourceCertPath,
		RemoteCertPath: remoteCertPath,
		Mode:           mode,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy Harbor cert from %s to %s", instanceName, sourceCertPath, remoteCertPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyHarborCertStepBuilder).Init(s)
}

func (s *CopyHarborCertStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyHarborCertStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CopyHarborCertStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Infof("Copying Harbor cert from %s to %s:%s", s.SourceCertPath, ctx.GetHost().GetName(), s.RemoteCertPath)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourceCertPath, s.RemoteCertPath, false, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to copy Harbor cert")
		return result, err
	}

	logger.Infof("Harbor cert copied successfully to %s", s.RemoteCertPath)
	result.MarkCompleted("Harbor cert copied")
	return result, nil
}

func (s *CopyHarborCertStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemoteCertPath)
	runner.Remove(ctx.GoContext(), conn, s.RemoteCertPath, true, false)
	return nil
}

var _ step.Step = (*CopyHarborCertStep)(nil)
