package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type LoadKernelModulesStep struct {
	step.Base
	// Add any necessary fields here
}

type LoadKernelModulesStepBuilder struct {
	step.Builder[LoadKernelModulesStepBuilder, *LoadKernelModulesStep]
}

func NewLoadKernelModulesStepBuilder(ctx runtime.Context, instanceName string) *LoadKernelModulesStepBuilder {
	s := &LoadKernelModulesStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Load kernel modules", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(LoadKernelModulesStepBuilder).Init(s)
	return b
}

func (s *LoadKernelModulesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *LoadKernelModulesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *LoadKernelModulesStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *LoadKernelModulesStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
