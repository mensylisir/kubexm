package pki

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EtcdCertDefinitionsStep struct {
	step.Base
	// Add any necessary fields here
}

type EtcdCertDefinitionsStepBuilder struct {
	step.Builder[EtcdCertDefinitionsStepBuilder, *EtcdCertDefinitionsStep]
}

func NewEtcdCertDefinitionsStepBuilder(ctx runtime.Context, instanceName string) *EtcdCertDefinitionsStepBuilder {
	s := &EtcdCertDefinitionsStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Define etcd certificate definitions", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(EtcdCertDefinitionsStepBuilder).Init(s)
	return b
}

func (s *EtcdCertDefinitionsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdCertDefinitionsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *EtcdCertDefinitionsStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *EtcdCertDefinitionsStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
