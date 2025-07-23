package pki

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DetermineEtcdPkiPathStep struct {
	step.Base
	// Add any necessary fields here
}

type DetermineEtcdPkiPathStepBuilder struct {
	step.Builder[DetermineEtcdPkiPathStepBuilder, *DetermineEtcdPkiPathStep]
}

func NewDetermineEtcdPkiPathStepBuilder(ctx runtime.Context, instanceName string) *DetermineEtcdPkiPathStepBuilder {
	s := &DetermineEtcdPkiPathStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Determine etcd pki path", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DetermineEtcdPkiPathStepBuilder).Init(s)
	return b
}

func (s *DetermineEtcdPkiPathStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DetermineEtcdPkiPathStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *DetermineEtcdPkiPathStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *DetermineEtcdPkiPathStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
