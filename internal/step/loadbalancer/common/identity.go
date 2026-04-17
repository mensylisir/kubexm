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

// LBIdentity represents load balancer identity (VIP, bind address, port).
type LBIdentity struct {
	VIP          string
	BindAddress  string
	FrontendPort int
	BackendPort  int
}

// CollectLBIdentityStep collects load balancer identity (VIP, bind address, frontend port).
type CollectLBIdentityStep struct {
	step.Base
}

type CollectLBIdentityStepBuilder struct {
	step.Builder[CollectLBIdentityStepBuilder, *CollectLBIdentityStep]
}

func NewCollectLBIdentityStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CollectLBIdentityStepBuilder {
	s := &CollectLBIdentityStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Collect LB identity (VIP, bind, port)", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CollectLBIdentityStepBuilder).Init(s)
}

func (s *CollectLBIdentityStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// GetIdentity returns the load balancer identity from cluster config.
func (s *CollectLBIdentityStep) GetIdentity(ctx runtime.ExecutionContext) (*LBIdentity, error) {
	cluster := ctx.GetClusterConfig()
	cpEndpoint := cluster.Spec.ControlPlaneEndpoint

	identity := &LBIdentity{
		VIP:          cpEndpoint.Address,
		BindAddress:  cpEndpoint.Address,
		FrontendPort: cpEndpoint.Port,
		BackendPort:  common.DefaultAPIServerPort,
	}

	// Override with HA config if present
	if cpEndpoint.HighAvailability != nil {
		// Try to get VIP from KubeVIP config
		if cpEndpoint.HighAvailability.External != nil &&
			cpEndpoint.HighAvailability.External.KubeVIP != nil &&
			cpEndpoint.HighAvailability.External.KubeVIP.VIP != nil &&
			*cpEndpoint.HighAvailability.External.KubeVIP.VIP != "" {
			identity.VIP = *cpEndpoint.HighAvailability.External.KubeVIP.VIP
			identity.BindAddress = identity.VIP
		}
		// Try to get VIP from Keepalived config
		if cpEndpoint.HighAvailability.External != nil &&
			cpEndpoint.HighAvailability.External.Keepalived != nil &&
			len(cpEndpoint.HighAvailability.External.Keepalived.VRRPInstances) > 0 &&
			len(cpEndpoint.HighAvailability.External.Keepalived.VRRPInstances[0].VirtualIPs) > 0 {
			vip := cpEndpoint.HighAvailability.External.Keepalived.VRRPInstances[0].VirtualIPs[0]
			if vip != "" {
				identity.VIP = vip
				identity.BindAddress = vip
			}
		}
	}

	return identity, nil
}

func (s *CollectLBIdentityStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	_, err := s.GetIdentity(ctx)
	return err == nil, err
}

func (s *CollectLBIdentityStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	identity, err := s.GetIdentity(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to collect LB identity")
		return result, err
	}

	logger.Infof("LB Identity: VIP=%s, Bind=%s, FrontendPort=%d, BackendPort=%d",
		identity.VIP, identity.BindAddress, identity.FrontendPort, identity.BackendPort)
	result.MarkCompleted("LB identity collected")
	return result, nil
}

func (s *CollectLBIdentityStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CollectLBIdentityStep)(nil)
