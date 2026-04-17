package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ExtractBundleStep extracts an offline bundle.
type ExtractBundleStep struct {
	step.Base
	BundlePath string
	TargetDir  string
}

type ExtractBundleStepBuilder struct {
	step.Builder[ExtractBundleStepBuilder, *ExtractBundleStep]
}

func NewExtractBundleStepBuilder(ctx runtime.ExecutionContext, instanceName, bundlePath, targetDir string) *ExtractBundleStepBuilder {
	s := &ExtractBundleStep{
		BundlePath: bundlePath,
		TargetDir:  targetDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract bundle from %s to %s", instanceName, bundlePath, targetDir)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	return new(ExtractBundleStepBuilder).Init(s)
}

func (s *ExtractBundleStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractBundleStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ExtractBundleStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("tar -xzf %s -C %s", s.BundlePath, s.TargetDir)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to extract bundle")
		return result, err
	}

	logger.Infof("Bundle extracted successfully to %s", s.TargetDir)
	result.MarkCompleted("Bundle extracted")
	return result, nil
}

func (s *ExtractBundleStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ExtractBundleStep)(nil)
