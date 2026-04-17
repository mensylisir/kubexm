package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// PushImageStep pushes an image to a registry.
type PushImageStep struct {
	step.Base
	ImageName string
}

type PushImageStepBuilder struct {
	step.Builder[PushImageStepBuilder, *PushImageStep]
}

func NewPushImageStepBuilder(ctx runtime.ExecutionContext, instanceName, imageName string) *PushImageStepBuilder {
	s := &PushImageStep{
		ImageName: imageName,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Push image %s", instanceName, imageName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	return new(PushImageStepBuilder).Init(s)
}

func (s *PushImageStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PushImageStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *PushImageStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("docker push %s", s.ImageName)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to push image")
		return result, err
	}

	logger.Infof("Image %s pushed successfully", s.ImageName)
	result.MarkCompleted("Image pushed")
	return result, nil
}

func (s *PushImageStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*PushImageStep)(nil)
