package pki

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type GenerateCaStep struct {
	step.Base
	// Add any necessary fields here
}

type GenerateCaStepBuilder struct {
	step.Builder[GenerateCaStepBuilder, *GenerateCaStep]
}

func NewGenerateCaStepBuilder(ctx runtime.Context, instanceName string) *GenerateCaStepBuilder {
	s := &GenerateCaStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate CA", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateCaStepBuilder).Init(s)
	return b
}

func (s *GenerateCaStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCaStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *GenerateCaStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *GenerateCaStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
