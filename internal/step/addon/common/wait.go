package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// WaitForAddonReadyStep waits for addon pods to be ready.
type WaitForAddonReadyStep struct {
	step.Base
	Namespace      string
	LabelSelector  string
	Timeout        time.Duration
	KubeconfigPath string
}

type WaitForAddonReadyStepBuilder struct {
	step.Builder[WaitForAddonReadyStepBuilder, *WaitForAddonReadyStep]
}

func NewWaitForAddonReadyStepBuilder(ctx runtime.ExecutionContext, instanceName, namespace, labelSelector, kubeconfigPath string) *WaitForAddonReadyStepBuilder {
	s := &WaitForAddonReadyStep{
		Namespace:      namespace,
		LabelSelector:  labelSelector,
		KubeconfigPath: kubeconfigPath,
		Timeout:        5 * time.Minute,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Wait for addon ready", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = s.Timeout
	return new(WaitForAddonReadyStepBuilder).Init(s)
}

func (b *WaitForAddonReadyStepBuilder) WithTimeout(timeout time.Duration) *WaitForAddonReadyStepBuilder {
	b.Step.Timeout = timeout
	return b
}

func (s *WaitForAddonReadyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *WaitForAddonReadyStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *WaitForAddonReadyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("kubectl wait --for=condition=Ready --timeout=%s pods -l %s -n %s --request-timeout=%s --kubeconfig %s",
		s.Timeout, s.LabelSelector, s.Namespace, s.Timeout, s.KubeconfigPath)

	logger.Infof("Waiting for addon: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "addon not ready")
		return result, err
	}

	logger.Infof("Addon is ready")
	result.MarkCompleted("Addon ready")
	return result, nil
}

func (s *WaitForAddonReadyStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*WaitForAddonReadyStep)(nil)
