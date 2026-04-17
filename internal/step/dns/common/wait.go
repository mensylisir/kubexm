package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// WaitForDNSReadyStep waits for DNS pods to be ready.
type WaitForDNSReadyStep struct {
	step.Base
	Namespace      string
	LabelSelector  string
	Timeout        time.Duration
	KubeconfigPath string
}

type WaitForDNSReadyStepBuilder struct {
	step.Builder[WaitForDNSReadyStepBuilder, *WaitForDNSReadyStep]
}

func NewWaitForDNSReadyStepBuilder(ctx runtime.ExecutionContext, instanceName, namespace, labelSelector, kubeconfigPath string) *WaitForDNSReadyStepBuilder {
	s := &WaitForDNSReadyStep{
		Namespace:      namespace,
		LabelSelector:  labelSelector,
		KubeconfigPath: kubeconfigPath,
		Timeout:        5 * time.Minute,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Wait for DNS to be ready", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = s.Timeout
	return new(WaitForDNSReadyStepBuilder).Init(s)
}

func (b *WaitForDNSReadyStepBuilder) WithTimeout(timeout time.Duration) *WaitForDNSReadyStepBuilder {
	b.Step.Timeout = timeout
	return b
}

func (s *WaitForDNSReadyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *WaitForDNSReadyStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *WaitForDNSReadyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	logger.Infof("Waiting for DNS: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "DNS not ready")
		return result, err
	}

	logger.Infof("DNS is ready")
	result.MarkCompleted("DNS ready")
	return result, nil
}

func (s *WaitForDNSReadyStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*WaitForDNSReadyStep)(nil)
