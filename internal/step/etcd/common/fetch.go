package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// FetchEtcdHealthStep checks etcd cluster or member health.
type FetchEtcdHealthStep struct {
	step.Base
	Endpoint  string
	UseSSL    bool
	Timeout   time.Duration
	HealthKey string // Context key to store health status
}

type FetchEtcdHealthStepBuilder struct {
	step.Builder[FetchEtcdHealthStepBuilder, *FetchEtcdHealthStep]
}

func NewFetchEtcdHealthStepBuilder(ctx runtime.ExecutionContext, instanceName, endpoint, healthKey string) *FetchEtcdHealthStepBuilder {
	s := &FetchEtcdHealthStep{
		Endpoint:  endpoint,
		UseSSL:    true,
		Timeout:   10 * time.Second,
		HealthKey: healthKey,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check etcd health for %s", instanceName, endpoint)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = s.Timeout + 5*time.Second
	return new(FetchEtcdHealthStepBuilder).Init(s)
}

func (b *FetchEtcdHealthStepBuilder) WithSSL(useSSL bool) *FetchEtcdHealthStepBuilder {
	b.Step.UseSSL = useSSL
	return b
}

func (b *FetchEtcdHealthStepBuilder) WithTimeout(timeout time.Duration) *FetchEtcdHealthStepBuilder {
	b.Step.Timeout = timeout
	return b
}

func (s *FetchEtcdHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *FetchEtcdHealthStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *FetchEtcdHealthStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	scheme := "http"
	if s.UseSSL {
		scheme = "https"
	}

	healthURL := fmt.Sprintf("%s://%s:%s/health", scheme, ctx.GetHost().GetAddress(), "2379")

	cmd := fmt.Sprintf("curl -s --connect-timeout %s %s/health", s.Timeout, healthURL)
	logger.Infof("Running: %s", cmd)

	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	if err != nil {
		result.MarkFailed(err, "etcd health check failed")
		return result, err
	}

	logger.Infof("Etcd health check result: %s", runResult.Stdout)
	ctx.Export("task", s.HealthKey, runResult.Stdout)

	result.MarkCompleted("Etcd health checked")
	return result, nil
}

func (s *FetchEtcdHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*FetchEtcdHealthStep)(nil)
