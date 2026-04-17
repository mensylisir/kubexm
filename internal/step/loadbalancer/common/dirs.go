package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// PrepareLBDirsStep creates necessary directories for load balancer on target hosts.
type PrepareLBDirsStep struct {
	step.Base
	Dirs []string
}

type PrepareLBDirsStepBuilder struct {
	step.Builder[PrepareLBDirsStepBuilder, *PrepareLBDirsStep]
}

func NewPrepareLBDirsStepBuilder(ctx runtime.ExecutionContext, instanceName string, dirs []string) *PrepareLBDirsStepBuilder {
	s := &PrepareLBDirsStep{Dirs: dirs}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Prepare directories for LB: %v", instanceName, dirs)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(PrepareLBDirsStepBuilder).Init(s)
}

func (s *PrepareLBDirsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PrepareLBDirsStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil // Always run to ensure directories exist
}

func (s *PrepareLBDirsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	for _, dir := range s.Dirs {
		if err := runner.Mkdirp(ctx.GoContext(), conn, dir, "0755", true); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to create directory %s", dir))
			return result, err
		}
		logger.Infof("Directory %s created successfully", dir)
	}

	result.MarkCompleted(fmt.Sprintf("Prepared %d directories", len(s.Dirs)))
	return result, nil
}

func (s *PrepareLBDirsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*PrepareLBDirsStep)(nil)
