package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// PrepareOSDirsStep creates necessary directories on the OS.
type PrepareOSDirsStep struct {
	step.Base
	Dirs      []string
	Recursive bool
	Owner     string
	Group     string
	Mode      string
}

type PrepareOSDirsStepBuilder struct {
	step.Builder[PrepareOSDirsStepBuilder, *PrepareOSDirsStep]
}

func NewPrepareOSDirsStepBuilder(ctx runtime.ExecutionContext, instanceName string, dirs []string) *PrepareOSDirsStepBuilder {
	s := &PrepareOSDirsStep{
		Dirs:      dirs,
		Recursive: true,
		Owner:     "root",
		Group:     "root",
		Mode:      "0755",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Prepare OS directories: %v", instanceName, dirs)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(PrepareOSDirsStepBuilder).Init(s)
}

func (b *PrepareOSDirsStepBuilder) WithRecursive(recursive bool) *PrepareOSDirsStepBuilder {
	b.Step.Recursive = recursive
	return b
}

func (b *PrepareOSDirsStepBuilder) WithOwner(owner, group string) *PrepareOSDirsStepBuilder {
	b.Step.Owner = owner
	b.Step.Group = group
	return b
}

func (b *PrepareOSDirsStepBuilder) WithMode(mode string) *PrepareOSDirsStepBuilder {
	b.Step.Mode = mode
	return b
}

func (s *PrepareOSDirsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PrepareOSDirsStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *PrepareOSDirsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	for _, dir := range s.Dirs {
		if err := runner.Mkdirp(ctx.GoContext(), conn, dir, s.Mode, s.Sudo); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to create directory %s", dir))
			return result, err
		}
		if s.Owner != "" && s.Group != "" {
			if err := runner.Chown(ctx.GoContext(), conn, dir, s.Owner, s.Group, s.Sudo); err != nil {
				logger.Warnf("Failed to set ownership on %s: %v", dir, err)
			}
		}
		logger.Infof("Directory %s created successfully", dir)
	}

	result.MarkCompleted(fmt.Sprintf("Prepared %d directories", len(s.Dirs)))
	return result, nil
}

func (s *PrepareOSDirsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*PrepareOSDirsStep)(nil)
