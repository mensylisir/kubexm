package pki

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type SetupEtcdPkiDataContextStep struct {
	step.Base
	// Add any necessary fields here
}

type SetupEtcdPkiDataContextStepBuilder struct {
	step.Builder[SetupEtcdPkiDataContextStepBuilder, *SetupEtcdPkiDataContextStep]
}

func NewSetupEtcdPkiDataContextStepBuilder(ctx runtime.Context, instanceName string) *SetupEtcdPkiDataContextStepBuilder {
	s := &SetupEtcdPkiDataContextStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup etcd pki data context", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(SetupEtcdPkiDataContextStepBuilder).Init(s)
	return b
}

func (s *SetupEtcdPkiDataContextStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupEtcdPkiDataContextStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *SetupEtcdPkiDataContextStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *SetupEtcdPkiDataContextStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
