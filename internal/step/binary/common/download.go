package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// DownloadBinaryStep downloads a binary from a URL.
type DownloadBinaryStep struct {
	step.Base
	URL         string
	TargetPath  string
	ExpectedSHA string
}

type DownloadBinaryStepBuilder struct {
	step.Builder[DownloadBinaryStepBuilder, *DownloadBinaryStep]
}

func NewDownloadBinaryStepBuilder(ctx runtime.ExecutionContext, instanceName, url, targetPath string) *DownloadBinaryStepBuilder {
	s := &DownloadBinaryStep{
		URL:        url,
		TargetPath: targetPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download binary from %s", instanceName, url)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	return new(DownloadBinaryStepBuilder).Init(s)
}

func (b *DownloadBinaryStepBuilder) WithExpectedSHA(sha string) *DownloadBinaryStepBuilder {
	b.Step.ExpectedSHA = sha
	return b
}

func (s *DownloadBinaryStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadBinaryStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *DownloadBinaryStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("curl -fsSL -o %s %s", s.TargetPath, s.URL)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to download binary")
		return result, err
	}

	logger.Infof("Binary downloaded successfully to %s", s.TargetPath)
	result.MarkCompleted("Binary downloaded")
	return result, nil
}

func (s *DownloadBinaryStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.TargetPath)
	runner.Remove(ctx.GoContext(), conn, s.TargetPath, true, false)
	return nil
}

var _ step.Step = (*DownloadBinaryStep)(nil)
