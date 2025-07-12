package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// WaitForPortStep waits for a port to become available on a target host.
type WaitForPortStep struct {
	meta    spec.StepMeta
	Host    string        // Target host
	Port    int           // Port to check
	Timeout time.Duration // How long to wait
}

// NewWaitForPortStep creates a new WaitForPortStep.
func NewWaitForPortStep(instanceName, host string, port int, timeout time.Duration) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("WaitForPort-%s:%d", host, port)
	}
	return &WaitForPortStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Waits for port %d to be available on %s", port, host),
		},
		Host:    host,
		Port:    port,
		Timeout: timeout,
	}
}

func (s *WaitForPortStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck checks if the port is already available.
func (s *WaitForPortStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Precheck")

	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Try to connect to the port with a short timeout
	available, err := runner.CheckPortAvailable(ctx.GoContext(), conn, s.Host, s.Port, 5*time.Second)
	if err != nil {
		logger.Debug("Port check failed", "error", err)
		return false, nil // Port not available yet
	}

	if available {
		logger.Debug("Port is already available", "host", s.Host, "port", s.Port)
		return true, nil
	}

	logger.Debug("Port is not yet available", "host", s.Host, "port", s.Port)
	return false, nil
}

// Run waits for the port to become available.
func (s *WaitForPortStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Run")

	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Waiting for port to become available", "host", s.Host, "port", s.Port, "timeout", s.Timeout)

	available, err := runner.CheckPortAvailable(ctx.GoContext(), conn, s.Host, s.Port, s.Timeout)
	if err != nil {
		logger.Error("Port availability check failed", "error", err)
		return fmt.Errorf("failed to check port %d on %s: %w", s.Port, s.Host, err)
	}

	if !available {
		return fmt.Errorf("port %d on %s did not become available within timeout %v", s.Port, s.Host, s.Timeout)
	}

	logger.Info("Port is now available", "host", s.Host, "port", s.Port)
	return nil
}

// Rollback is a no-op for WaitForPortStep.
func (s *WaitForPortStep) Rollback(ctx step.StepContext, host connector.Host) error {
	return nil
}

var _ step.Step = (*WaitForPortStep)(nil)