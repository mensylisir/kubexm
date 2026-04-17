package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyRegistryCertStep copies registry certificates to remote host.
type CopyRegistryCertStep struct {
	step.Base
	SourceCertPath string
	RemoteCertPath string
	Mode           string
}

type CopyRegistryCertStepBuilder struct {
	step.Builder[CopyRegistryCertStepBuilder, *CopyRegistryCertStep]
}

func NewCopyRegistryCertStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceCertPath, remoteCertPath, mode string) *CopyRegistryCertStepBuilder {
	s := &CopyRegistryCertStep{
		SourceCertPath: sourceCertPath,
		RemoteCertPath: remoteCertPath,
		Mode:           mode,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy registry cert from %s to %s", instanceName, sourceCertPath, remoteCertPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyRegistryCertStepBuilder).Init(s)
}

func (s *CopyRegistryCertStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyRegistryCertStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CopyRegistryCertStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Infof("Copying registry cert from %s to %s:%s", s.SourceCertPath, ctx.GetHost().GetName(), s.RemoteCertPath)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourceCertPath, s.RemoteCertPath, false, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to copy registry cert")
		return result, err
	}

	logger.Infof("Registry cert copied successfully to %s", s.RemoteCertPath)
	result.MarkCompleted("Registry cert copied")
	return result, nil
}

func (s *CopyRegistryCertStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*CopyRegistryCertStep)(nil)
