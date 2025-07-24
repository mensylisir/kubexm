package network

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanNetworkStep struct {
	step.Base
	// Add any necessary fields here
}

type CleanNetworkStepBuilder struct {
	step.Builder[CleanNetworkStepBuilder, *CleanNetworkStep]
}

func NewCleanNetworkStepBuilder(ctx runtime.Context, instanceName string) *CleanNetworkStepBuilder {
	s := &CleanNetworkStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean network configurations", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanNetworkStepBuilder).Init(s)
	return b
}

func (s *CleanNetworkStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanNetworkStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *CleanNetworkStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *CleanNetworkStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
