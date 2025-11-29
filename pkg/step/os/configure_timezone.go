package os

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ConfigureTimezoneStep struct {
	step.Base
	Timezone string
}

type ConfigureTimezoneStepBuilder struct {
	step.Builder[ConfigureTimezoneStepBuilder, *ConfigureTimezoneStep]
}

func NewConfigureTimezoneStepBuilder(ctx runtime.ExecutionContext, instanceName string) *ConfigureTimezoneStepBuilder {
	var timezone string
	if ctx.GetClusterConfig().Spec.System != nil && ctx.GetClusterConfig().Spec.System.Timezone != "" {
		timezone = ctx.GetClusterConfig().Spec.System.Timezone
	}

	s := &ConfigureTimezoneStep{
		Timezone: timezone,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Configure the system timezone"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureTimezoneStepBuilder).Init(s)
	return b
}

func (s *ConfigureTimezoneStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureTimezoneStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for system timezone configuration...")

	if s.Timezone == "" {
		logger.Info("No timezone specified in configuration. Step is done.")
		return true, nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	getCurrentTimezoneCmd := "timedatectl | grep 'Time zone' | awk '{print $3}'"
	stdout, err := runner.Run(ctx.GoContext(), conn, getCurrentTimezoneCmd, s.Sudo)
	if err != nil {
		logger.Warnf("Failed to get current timezone, will attempt to set it. Error: %v", err)
		return false, nil
	}

	currentTimezone := strings.TrimSpace(string(stdout))

	if currentTimezone == s.Timezone {
		logger.Infof("Precheck: System timezone is already set to '%s'. Step is done.", s.Timezone)
		return true, nil
	}

	logger.Infof("Precheck passed: Current timezone is '%s', desired is '%s'. Configuration is needed.", currentTimezone, s.Timezone)
	return false, nil
}

func (s *ConfigureTimezoneStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if s.Timezone == "" {
		logger.Info("No timezone specified in configuration. Nothing to do.")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Setting system timezone to: %s", s.Timezone)
	setTimezoneCmd := fmt.Sprintf("timedatectl set-timezone %s", s.Timezone)

	if _, err := runner.Run(ctx.GoContext(), conn, setTimezoneCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to set timezone to '%s': %w", s.Timezone, err)
	}

	logger.Info("System timezone configured successfully.")
	return nil
}

func (s *ConfigureTimezoneStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for timezone configuration is not implemented.")
	return nil
}

var _ step.Step = (*ConfigureTimezoneStep)(nil)
