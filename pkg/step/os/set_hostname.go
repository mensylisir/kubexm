package os

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type SetHostnameStep struct {
	step.Base
	Hostname string
}

type SetHostnameStepBuilder struct {
	step.Builder[SetHostnameStepBuilder, *SetHostnameStep]
}

func NewSetHostnameStepBuilder(ctx runtime.ExecutionContext, instanceName, hostname string) *SetHostnameStepBuilder {
	cs := &SetHostnameStep{
		Hostname: hostname,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Set hostname to [%s]", instanceName, hostname)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 1 * time.Minute
	return new(SetHostnameStepBuilder).Init(cs)
}

func (s *SetHostnameStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetHostnameStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector: %w", err)
	}

	output, err := runnerSvc.Run(ctx.GoContext(), conn, "hostname", false)
	if err != nil {
		logger.Warn(err, "Failed to get current hostname, assuming it needs to be set.")
		return false, nil
	}

	currentHostname := strings.TrimSpace(string(output))
	if currentHostname == s.Hostname {
		logger.Info("Hostname is already set to desired value.", "hostname", s.Hostname)
		return true, nil
	}

	logger.Info("Current hostname does not match desired value. Step needs to run.", "current", currentHostname, "desired", s.Hostname)
	return false, nil
}

func (s *SetHostnameStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector: %w", err)
	}

	logger.Info("Setting hostname.", "hostname", s.Hostname)
	err = runnerSvc.SetHostname(ctx.GoContext(), conn, nil, s.Hostname)
	if err != nil {
		return fmt.Errorf("failed to set hostname to '%s': %w", s.Hostname, err)
	}

	logger.Info("Hostname set successfully.", "hostname", s.Hostname)
	return nil
}

func (s *SetHostnameStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for SetHostnameStep is a no-op. Changing hostname back is not recommended during a failed deployment.")
	return nil
}

var _ step.Step = (*SetHostnameStep)(nil)
