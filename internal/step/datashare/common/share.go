package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ShareDataStep shares data between hosts.
type ShareDataStep struct {
	step.Base
	SourcePath string
	TargetPath string
}

type ShareDataStepBuilder struct {
	step.Builder[ShareDataStepBuilder, *ShareDataStep]
}

func NewShareDataStepBuilder(ctx runtime.ExecutionContext, instanceName, sourcePath, targetPath string) *ShareDataStepBuilder {
	s := &ShareDataStep{
		SourcePath: sourcePath,
		TargetPath: targetPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Share data from %s to %s", instanceName, sourcePath, targetPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(ShareDataStepBuilder).Init(s)
}

func (s *ShareDataStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ShareDataStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ShareDataStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Infof("Sharing data from %s to %s recursively", s.SourcePath, s.TargetPath)

	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourcePath, s.TargetPath, true, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to share data")
		return result, err
	}

	logger.Infof("Data shared successfully to %s", s.TargetPath)
	result.MarkCompleted("Data shared")
	return result, nil
}

func (s *ShareDataStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ShareDataStep)(nil)
