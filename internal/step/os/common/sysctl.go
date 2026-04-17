package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// SysctlParam represents a single sysctl parameter.
type SysctlParam struct {
	Key   string
	Value string
}

// ApplySysctlStep applies sysctl parameters.
type ApplySysctlStep struct {
	step.Base
	Params    []SysctlParam
	Permanent bool
}

type ApplySysctlStepBuilder struct {
	step.Builder[ApplySysctlStepBuilder, *ApplySysctlStep]
}

func NewApplySysctlStepBuilder(ctx runtime.ExecutionContext, instanceName string, params []SysctlParam) *ApplySysctlStepBuilder {
	s := &ApplySysctlStep{
		Params:    params,
		Permanent: false,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply sysctl parameters", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(ApplySysctlStepBuilder).Init(s)
}

func (b *ApplySysctlStepBuilder) WithPermanent(permanent bool) *ApplySysctlStepBuilder {
	b.Step.Permanent = permanent
	return b
}

func (s *ApplySysctlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ApplySysctlStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ApplySysctlStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	for _, param := range s.Params {
		cmd := fmt.Sprintf("sysctl -w %s=%s", param.Key, param.Value)
		logger.Infof("Running: %s", cmd)
		if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to apply sysctl %s", param.Key))
			return result, err
		}

		if s.Permanent {
			// Add to /etc/sysctl.d/99-kubexm.conf
			sysctlConfPath := "/etc/sysctl.d/99-kubexm.conf"
			sysctlLine := fmt.Sprintf("%s = %s\n", param.Key, param.Value)
			appendCmd := fmt.Sprintf("echo '%s' >> %s", sysctlLine, sysctlConfPath)
			if _, err := runner.Run(ctx.GoContext(), conn, appendCmd, s.Sudo); err != nil {
				logger.Warnf("Failed to make %s permanent: %v", param.Key, err)
			}
		}
	}

	result.MarkCompleted(fmt.Sprintf("Applied %d sysctl parameters", len(s.Params)))
	return result, nil
}

func (s *ApplySysctlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	for _, param := range s.Params {
		// Try to reset to default
		cmd := fmt.Sprintf("sysctl -w %s=0", param.Key)
		logger.Warnf("Rolling back by resetting %s", param.Key)
		runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	}
	return nil
}

var _ step.Step = (*ApplySysctlStep)(nil)

// ParseSysctlParams parses sysctl parameters from a map.
func ParseSysctlParams(params map[string]string) []SysctlParam {
	var result []SysctlParam
	for key, value := range params {
		result = append(result, SysctlParam{Key: key, Value: value})
	}
	return result
}

// CommonSysctlParams returns common sysctl parameters for Kubernetes.
func CommonSysctlParams() []SysctlParam {
	return []SysctlParam{
		{Key: "net.bridge.bridge-nf-call-iptables", Value: "1"},
		{Key: "net.bridge.bridge-nf-call-ip6tables", Value: "1"},
		{Key: "net.ipv4.ip_forward", Value: "1"},
		{Key: "net.ipv4.conf.all.forwarding", Value: "1"},
		{Key: "net.ipv4.conf.default.forwarding", Value: "1"},
	}
}
