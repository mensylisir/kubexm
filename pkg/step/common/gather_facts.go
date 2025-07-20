package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

type GatherFactsStep struct {
	step.Base
}

type GatherFactsStepBuilder struct {
	step.Builder[GatherFactsStepBuilder, *GatherFactsStep]
}

func NewGatherFactsStepBuilder(instanceName string) *GatherFactsStepBuilder {
	cs := &GatherFactsStep{}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Gather osinfo", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(GatherFactsStepBuilder).Init(cs)
}

func (s *GatherFactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GatherFactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	facts, err := runnerSvc.GatherHostFacts(ctx.GoContext(), conn)
	if err != nil || facts == nil {
		logger.Debug("Facts not yet collected for host")
		return false, nil
	}
	if facts.OS.PrettyName == "" || facts.CPU.Architecture == "" {
		logger.Debug("Incomplete facts for host, will re-gather")
		return false, nil
	}
	logger.Debug("Facts already available for host", "os", facts.OS.PrettyName, "arch", facts.CPU.Architecture)
	return true, nil
}

func (s *GatherFactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	logger.Info("Gathering facts from host")
	facts, err := runner.GatherHostFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Error("Failed to gather facts", "error", err)
		return fmt.Errorf("failed to gather facts from host %s: %w", ctx.GetHost().GetName(), err)
	}

	cores := facts.CPU.CoresPerSocket * facts.CPU.Sockets

	logger.Info("Successfully gathered facts",
		"os", facts.OS.PrettyName,
		"arch", facts.CPU.Architecture,
		"memory", facts.Memory.Total,
		"cores", cores)

	cache := ctx.GetTaskCache()
	cacheKey := fmt.Sprintf(common.CacheKeyHostFactsTemplate, ctx.GetHost().GetName())
	cache.Set(cacheKey, facts)
	return nil
}
func (s *GatherFactsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*GatherFactsStep)(nil)
