package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// SaveImageStep saves an image to a tar file.
type SaveImageStep struct {
	step.Base
	ImageName  string
	OutputPath string
}

type SaveImageStepBuilder struct {
	step.Builder[SaveImageStepBuilder, *SaveImageStep]
}

func NewSaveImageStepBuilder(ctx runtime.ExecutionContext, instanceName, imageName, outputPath string) *SaveImageStepBuilder {
	s := &SaveImageStep{
		ImageName:  imageName,
		OutputPath: outputPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Save image %s to %s", instanceName, imageName, outputPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	return new(SaveImageStepBuilder).Init(s)
}

func (s *SaveImageStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SaveImageStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *SaveImageStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("docker save -o %s %s", s.OutputPath, s.ImageName)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to save image")
		return result, err
	}

	logger.Infof("Image %s saved to %s", s.ImageName, s.OutputPath)
	result.MarkCompleted("Image saved")
	return result, nil
}

func (s *SaveImageStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*SaveImageStep)(nil)
