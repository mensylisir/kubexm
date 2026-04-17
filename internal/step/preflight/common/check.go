package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CheckPortConnectivityStep checks if a port is reachable on a target host.
type CheckPortConnectivityStep struct {
	step.Base
	TargetHost  string
	TargetPort  int
	TimeoutSecs int
}

type CheckPortConnectivityStepBuilder struct {
	step.Builder[CheckPortConnectivityStepBuilder, *CheckPortConnectivityStep]
}

func NewCheckPortConnectivityStepBuilder(ctx runtime.ExecutionContext, instanceName, targetHost string, targetPort int) *CheckPortConnectivityStepBuilder {
	s := &CheckPortConnectivityStep{
		TargetHost:  targetHost,
		TargetPort:  targetPort,
		TimeoutSecs: 1,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check connectivity to %s:%d", instanceName, targetHost, targetPort)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CheckPortConnectivityStepBuilder).Init(s)
}

func (b *CheckPortConnectivityStepBuilder) WithTimeoutSecs(timeout int) *CheckPortConnectivityStepBuilder {
	b.Step.TimeoutSecs = timeout
	return b
}

func (s *CheckPortConnectivityStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckPortConnectivityStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CheckPortConnectivityStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("nc -z -w %d %s %d", s.TimeoutSecs, s.TargetHost, s.TargetPort)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("cannot connect to %s:%d", s.TargetHost, s.TargetPort))
		return result, err
	}

	logger.Infof("Port %d is reachable on %s", s.TargetPort, s.TargetHost)
	result.MarkCompleted(fmt.Sprintf("Port %d reachable", s.TargetPort))
	return result, nil
}

func (s *CheckPortConnectivityStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CheckPortConnectivityStep)(nil)
