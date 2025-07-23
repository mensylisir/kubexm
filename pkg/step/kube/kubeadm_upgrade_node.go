package kube

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmUpgradeNodeStep struct {
	step.Base
	// Add any necessary fields here
}

type KubeadmUpgradeNodeStepBuilder struct {
	step.Builder[KubeadmUpgradeNodeStepBuilder, *KubeadmUpgradeNodeStep]
}

func NewKubeadmUpgradeNodeStepBuilder(ctx runtime.Context, instanceName string) *KubeadmUpgradeNodeStepBuilder {
	s := &KubeadmUpgradeNodeStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Upgrade node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 10 * time.Minute

	b := new(KubeadmUpgradeNodeStepBuilder).Init(s)
	return b
}

func (s *KubeadmUpgradeNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmUpgradeNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *KubeadmUpgradeNodeStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *KubeadmUpgradeNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
