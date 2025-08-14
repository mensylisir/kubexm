package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"time"

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type PrintMessageStep struct {
	step.Base
	Message string
}

type PrintMessageStepBuilder struct {
	step.Builder[PrintMessageStepBuilder, *PrintMessageStep]
}

func NewPrintMessageStepBuilder(ctx runtime.ExecutionContext, instanceName string) *PrintMessageStepBuilder {
	cs := &PrintMessageStep{
		Message: common.DefaultLogo,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Print Logo: [%s]", instanceName, cs.Message)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(PrintMessageStepBuilder).Init(cs)
}

func (s *PrintMessageStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PrintMessageStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	_ = ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	return false, nil
}

func (s *PrintMessageStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info(s.Message)
	return nil
}

func (s *PrintMessageStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back PrintMessageStep is a no-op.")
	return nil
}

var _ step.Step = (*PrintMessageStep)(nil)
