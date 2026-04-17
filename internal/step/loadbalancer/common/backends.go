package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CollectLBBackendsStep collects backend server information (master nodes) for load balancer.
type CollectLBBackendsStep struct {
	step.Base
	TargetRole  string
	BackendPort int
}

type CollectLBBackendsStepBuilder struct {
	step.Builder[CollectLBBackendsStepBuilder, *CollectLBBackendsStep]
}

func NewCollectLBBackendsStepBuilder(ctx runtime.ExecutionContext, instanceName string, targetRole string, backendPort int) *CollectLBBackendsStepBuilder {
	s := &CollectLBBackendsStep{
		TargetRole:  targetRole,
		BackendPort: backendPort,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Collect %s backends", instanceName, targetRole)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CollectLBBackendsStepBuilder).Init(s)
}

func (s *CollectLBBackendsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// BackendServer represents a backend server for load balancer.
type BackendServer struct {
	Name    string
	Address string
	Port    int
}

// GetBackends returns the collected backend servers from context.
func (s *CollectLBBackendsStep) GetBackends(ctx runtime.ExecutionContext) ([]BackendServer, error) {
	var backends []BackendServer

	hosts := ctx.GetHostsByRole(s.TargetRole)
	for _, h := range hosts {
		backends = append(backends, BackendServer{
			Name:    h.GetName(),
			Address: h.GetInternalAddress(),
			Port:    s.BackendPort,
		})
	}

	return backends, nil
}

func (s *CollectLBBackendsStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	backends, err := s.GetBackends(ctx)
	if err != nil {
		return false, err
	}
	if len(backends) == 0 {
		return false, fmt.Errorf("no %s hosts found for backends", s.TargetRole)
	}
	return false, nil // Always render to ensure up-to-date
}

func (s *CollectLBBackendsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	backends, err := s.GetBackends(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to collect backends")
		return result, err
	}

	logger.Infof("Collected %d backend servers: %+v", len(backends), backends)
	result.MarkCompleted(fmt.Sprintf("Collected %d %s backends", len(backends), s.TargetRole))
	return result, nil
}

func (s *CollectLBBackendsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CollectLBBackendsStep)(nil)

// CollectMasterBackends is a convenience function to collect master nodes as backends.
func CollectMasterBackends(ctx runtime.ExecutionContext, instanceName string) (*CollectLBBackendsStep, error) {
	builder := NewCollectLBBackendsStepBuilder(ctx, instanceName, common.RoleMaster, common.DefaultAPIServerPort)
	return builder.Build()
}
