package common

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyPKICertStep copies a PKI certificate file to remote host.
type CopyPKICertStep struct {
	step.Base
	SourcePath   string
	RemotePath   string
	CertDir      string
	CertFileName string
	Mode         string
	Overwrite    bool
}

type CopyPKICertStepBuilder struct {
	step.Builder[CopyPKICertStepBuilder, *CopyPKICertStep]
}

func NewCopyPKICertStepBuilder(ctx runtime.ExecutionContext, instanceName, sourcePath, remoteDir, certFileName, mode string) *CopyPKICertStepBuilder {
	s := &CopyPKICertStep{
		SourcePath:   sourcePath,
		RemotePath:   filepath.Join(remoteDir, certFileName),
		CertDir:      remoteDir,
		CertFileName: certFileName,
		Mode:         mode,
		Overwrite:    true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy PKI cert %s to %s", instanceName, certFileName, remoteDir)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyPKICertStepBuilder).Init(s)
}

func (s *CopyPKICertStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyPKICertStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CopyPKICertStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.CertDir, "0755", s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to create cert directory")
		return result, err
	}

	logger.Infof("Copying PKI cert from %s to %s:%s", s.SourcePath, ctx.GetHost().GetName(), s.RemotePath)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourcePath, s.RemotePath, false, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to copy PKI cert")
		return result, err
	}

	logger.Infof("PKI cert copied successfully to %s", s.RemotePath)
	result.MarkCompleted(fmt.Sprintf("PKI cert %s copied", s.CertFileName))
	return result, nil
}

func (s *CopyPKICertStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemotePath)
	runner.Remove(ctx.GoContext(), conn, s.RemotePath, true, false)
	return nil
}

var _ step.Step = (*CopyPKICertStep)(nil)
