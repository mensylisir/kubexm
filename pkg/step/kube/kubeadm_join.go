package kube

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmJoinStep struct {
	step.Base
	// Add any necessary fields here
}

type KubeadmJoinStepBuilder struct {
	step.Builder[KubeadmJoinStepBuilder, *KubeadmJoinStep]
}

func NewKubeadmJoinStepBuilder(ctx runtime.Context, instanceName string) *KubeadmJoinStepBuilder {
	s := &KubeadmJoinStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Join node to cluster", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 10 * time.Minute

	b := new(KubeadmJoinStepBuilder).Init(s)
	return b
}

func (s *KubeadmJoinStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmJoinStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *KubeadmJoinStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *KubeadmJoinStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
