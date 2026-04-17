package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CompressBundleStep compresses files into a bundle.
type CompressBundleStep struct {
	step.Base
	SourceDir  string
	OutputPath string
}

type CompressBundleStepBuilder struct {
	step.Builder[CompressBundleStepBuilder, *CompressBundleStep]
}

func NewCompressBundleStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceDir, outputPath string) *CompressBundleStepBuilder {
	s := &CompressBundleStep{
		SourceDir:  sourceDir,
		OutputPath: outputPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Compress bundle from %s to %s", instanceName, sourceDir, outputPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	return new(CompressBundleStepBuilder).Init(s)
}

func (s *CompressBundleStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CompressBundleStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CompressBundleStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("tar -czf %s -C %s .", s.OutputPath, s.SourceDir)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to compress bundle")
		return result, err
	}

	logger.Infof("Bundle compressed successfully to %s", s.OutputPath)
	result.MarkCompleted("Bundle compressed")
	return result, nil
}

func (s *CompressBundleStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CompressBundleStep)(nil)
