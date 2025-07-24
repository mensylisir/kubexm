package pki

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type GenerateSignedCertStep struct {
	step.Base
	// Add any necessary fields here
}

type GenerateSignedCertStepBuilder struct {
	step.Builder[GenerateSignedCertStepBuilder, *GenerateSignedCertStep]
}

func NewGenerateSignedCertStepBuilder(ctx runtime.Context, instanceName string) *GenerateSignedCertStepBuilder {
	s := &GenerateSignedCertStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate signed certificate", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateSignedCertStepBuilder).Init(s)
	return b
}

func (s *GenerateSignedCertStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateSignedCertStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *GenerateSignedCertStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *GenerateSignedCertStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
