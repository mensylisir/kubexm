package kube

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmResetStep struct {
	step.Base
	// Add any necessary fields here
}

type KubeadmResetStepBuilder struct {
	step.Builder[KubeadmResetStepBuilder, *KubeadmResetStep]
}

func NewKubeadmResetStepBuilder(ctx runtime.Context, instanceName string) *KubeadmResetStepBuilder {
	s := &KubeadmResetStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Reset node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 10 * time.Minute

	b := new(KubeadmResetStepBuilder).Init(s)
	return b
}

func (s *KubeadmResetStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmResetStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *KubeadmResetStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *KubeadmResetStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
