package common

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// GatherFactsStep collects system information from target hosts.
type GatherFactsStep struct {
	meta spec.StepMeta
}

// NewGatherFactsStep creates a new GatherFactsStep.
func NewGatherFactsStep(instanceName string) step.Step {
	name := instanceName
	if name == "" {
		name = "GatherFacts"
	}
	return &GatherFactsStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Gathers system facts from target hosts",
		},
	}
}

func (s *GatherFactsStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck checks if facts are already collected for this host.
func (s *GatherFactsStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Precheck")

	// Check if facts are already available in the context
	facts, err := ctx.GetHostFacts(host)
	if err != nil || facts == nil {
		logger.Debug("Facts not yet collected for host")
		return false, nil
	}

	// Basic validation of facts - ensure we have core information
	if facts.OS.PrettyName == "" || facts.Hardware.CPU.Architecture == "" {
		logger.Debug("Incomplete facts for host, will re-gather")
		return false, nil
	}

	logger.Debug("Facts already available for host", "os", facts.OS.PrettyName, "arch", facts.Hardware.CPU.Architecture)
	return true, nil
}

// Run gathers facts from the target host.
func (s *GatherFactsStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Run")

	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Gathering facts from host")

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Error("Failed to gather facts", "error", err)
		return fmt.Errorf("failed to gather facts from host %s: %w", host.GetName(), err)
	}

	logger.Info("Successfully gathered facts", 
		"os", facts.OS.PrettyName, 
		"arch", facts.Hardware.CPU.Architecture,
		"memory", facts.Hardware.Memory.Total,
		"cores", facts.Hardware.CPU.Cores)

	// Store facts in cache for other steps to use
	cache := ctx.GetStepCache()
	cacheKey := fmt.Sprintf("facts_%s", host.GetName())
	cache.Set(cacheKey, facts)

	// Also store in pipeline cache for broader access
	pipelineCache := ctx.GetModuleCache()
	if pipelineCache != nil {
		pipelineCache.Set(cacheKey, facts)
	}

	return nil
}

// Rollback is a no-op for GatherFactsStep as fact gathering is read-only.
func (s *GatherFactsStep) Rollback(ctx step.StepContext, host connector.Host) error {
	return nil
}

var _ step.Step = (*GatherFactsStep)(nil)