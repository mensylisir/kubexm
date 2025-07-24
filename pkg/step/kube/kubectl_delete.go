package kube

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubectlDeleteStep struct {
	step.Base
	// Add any necessary fields here
}

type KubectlDeleteStepBuilder struct {
	step.Builder[KubectlDeleteStepBuilder, *KubectlDeleteStep]
}

func NewKubectlDeleteStepBuilder(ctx runtime.Context, instanceName string) *KubectlDeleteStepBuilder {
	s := &KubectlDeleteStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Delete resource with kubectl", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 10 * time.Minute

	b := new(KubectlDeleteStepBuilder).Init(s)
	return b
}

func (s *KubectlDeleteStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubectlDeleteStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *KubectlDeleteStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *KubectlDeleteStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
