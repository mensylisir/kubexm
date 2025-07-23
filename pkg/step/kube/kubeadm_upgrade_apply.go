package kube

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmUpgradeApplyStep struct {
	step.Base
	// Add any necessary fields here
}

type KubeadmUpgradeApplyStepBuilder struct {
	step.Builder[KubeadmUpgradeApplyStepBuilder, *KubeadmUpgradeApplyStep]
}

func NewKubeadmUpgradeApplyStepBuilder(ctx runtime.Context, instanceName string) *KubeadmUpgradeApplyStepBuilder {
	s := &KubeadmUpgradeApplyStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply upgrade", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 10 * time.Minute

	b := new(KubeadmUpgradeApplyStepBuilder).Init(s)
	return b
}

func (s *KubeadmUpgradeApplyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmUpgradeApplyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *KubeadmUpgradeApplyStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *KubeadmUpgradeApplyStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
