package common

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CheckConnectivityStep checks SSH connectivity to a host.
type CheckConnectivityStep struct {
	step.Base
	Host remotefw.Host
}

// NewCheckConnectivityStep creates a new CheckConnectivityStep.
func NewCheckConnectivityStep(host remotefw.Host) *CheckConnectivityStep {
	return &CheckConnectivityStep{
		Base: step.Base{
			Meta: spec.StepMeta{
				Name:        "CheckConnectivity",
				Description: fmt.Sprintf("Check SSH connectivity to %s", host.GetAddress()),
			},
		},
		Host: host,
	}
}

// Meta returns the step metadata.
func (s *CheckConnectivityStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck always returns false (no precheck needed).
func (s *CheckConnectivityStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

// Run executes the connectivity check.
func (s *CheckConnectivityStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), s.Host)
	log := ctx.GetLogger()

	addr := net.JoinHostPort(s.Host.GetAddress(), strconv.Itoa(s.Host.GetPort()))

	// Check TCP connection (SSH port)
	timeout := 5 * time.Second
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	if err != nil {
		log.Error(err, "Failed to connect", "host", s.Host.GetAddress(), "port", s.Host.GetPort())
		result.MarkFailed(err, fmt.Sprintf("cannot connect to %s:%d", s.Host.GetAddress(), s.Host.GetPort()))
		return result, err
	}
	conn.Close()

	log.Info("Host reachable", "host", s.Host.GetAddress(), "port", s.Host.GetPort(), "elapsed", elapsed)
	result.MarkCompleted(fmt.Sprintf("Connected to %s:%d in %v", s.Host.GetAddress(), s.Host.GetPort(), elapsed))
	return result, nil
}

// Rollback is a no-op for connectivity check.
func (s *CheckConnectivityStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

// String returns a string representation of the step.
func (s *CheckConnectivityStep) String() string {
	return fmt.Sprintf("CheckConnectivity(host=%s:%d)", s.Host.GetAddress(), s.Host.GetPort())
}

// Ensure Step interface is implemented.
var _ step.Step = (*CheckConnectivityStep)(nil)

// CheckConnectivityStepBuilder creates CheckConnectivityStep instances.
type CheckConnectivityStepBuilder struct {
	host remotefw.Host
}

// NewCheckConnectivityStepBuilder creates a new builder.
func NewCheckConnectivityStepBuilder(host remotefw.Host) *CheckConnectivityStepBuilder {
	return &CheckConnectivityStepBuilder{host: host}
}

// Build creates the CheckConnectivityStep.
func (b *CheckConnectivityStepBuilder) Build() (step.Step, error) {
	if b.host == nil {
		return nil, fmt.Errorf("host cannot be nil")
	}
	return NewCheckConnectivityStep(b.host), nil
}