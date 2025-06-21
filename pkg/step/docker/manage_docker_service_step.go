package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/containerd" // For ServiceAction type
)

const DockerServiceName = "docker"

// ManageDockerServiceStep performs a systemctl action on the Docker service.
type ManageDockerServiceStep struct {
	meta        spec.StepMeta
	Action      containerd.ServiceAction // Reusing ServiceAction from containerd steps
	ServiceName string                   // Defaults to "docker"
	Sudo        bool
}

// NewManageDockerServiceStep creates a new ManageDockerServiceStep.
func NewManageDockerServiceStep(instanceName string, action containerd.ServiceAction, sudo bool) step.Step {
	name := instanceName
	svcName := DockerServiceName

	if name == "" {
		name = fmt.Sprintf("ManageDockerService-%s", strings.Title(string(action)))
	}

	return &ManageDockerServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Performs systemctl %s on %s service.", action, svcName),
		},
		Action:      action,
		ServiceName: svcName,
		Sudo:        true,
	}
}

func (s *ManageDockerServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ManageDockerServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	switch s.Action {
	case containerd.ActionDaemonReload:
		logger.Info("Daemon-reload action does not have a direct precheck state, will always run if scheduled.")
		return false, nil
	case containerd.ActionStart, containerd.ActionRestart:
		active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
		if err != nil {
			logger.Warn("Failed to check if service is active, assuming it needs action.", "error", err)
			return false, nil
		}
		if s.Action == containerd.ActionStart && active {
			logger.Info("Service is already active, start action satisfied.")
			return true, nil
		}
		if s.Action == containerd.ActionRestart { // Restart always runs if scheduled
			return false, nil
		}
		return false, nil
	case containerd.ActionStop:
		active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
		if err != nil {
			logger.Warn("Failed to check if service is active for stop, assuming it needs action.", "error", err)
			return false, nil
		}
		if !active {
			logger.Info("Service is already stopped, stop action satisfied.")
			return true, nil
		}
		return false, nil
	case containerd.ActionEnable:
		cmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		stdout, _, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, false)
		if err != nil {
			logger.Warn("Failed to check if service is enabled, assuming it needs action.", "error", err)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) == "enabled" {
			logger.Info("Service is already enabled.")
			return true, nil
		}
		return false, nil
	case containerd.ActionDisable:
		cmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		stdout, _, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, false)
		if err != nil {
			logger.Warn("Failed to check if service is enabled for disable, assuming it needs action.", "error", err)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) != "enabled" {
			logger.Info("Service is already not enabled (or static), disable action satisfied.")
			return true, nil
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported service action for precheck: %s", s.Action)
	}
}

func (s *ManageDockerServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}
	facts, err := ctx.GetHostFacts(host) // Needed for some runner service methods
	if err != nil {
		return fmt.Errorf("failed to get host facts: %w", err)
	}


	var cmdErr error
	switch s.Action {
	case containerd.ActionStart:
		cmdErr = runnerSvc.StartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case containerd.ActionStop:
		cmdErr = runnerSvc.StopService(ctx.GoContext(), conn, facts, s.ServiceName)
	case containerd.ActionRestart:
		cmdErr = runnerSvc.RestartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case containerd.ActionEnable:
		cmdErr = runnerSvc.EnableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case containerd.ActionDisable:
		cmdErr = runnerSvc.DisableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case containerd.ActionDaemonReload:
		cmdErr = runnerSvc.DaemonReload(ctx.GoContext(), conn, facts)
	default:
		return fmt.Errorf("unsupported service action: %s", s.Action)
	}

	if cmdErr != nil {
		logger.Error("Systemctl command failed.", "action", s.Action, "service", s.ServiceName, "error", cmdErr)
		return fmt.Errorf("systemctl %s %s failed: %w", s.Action, s.ServiceName, cmdErr)
	}

	logger.Info("Systemctl command executed successfully.", "action", s.Action, "service", s.ServiceName)
	return nil
}

func (s *ManageDockerServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for ManageDockerServiceStep is generally not implemented or is context-specific.", "action", s.Action)
	return nil
}

var _ step.Step = (*ManageDockerServiceStep)(nil)
