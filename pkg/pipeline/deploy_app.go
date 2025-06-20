package pipeline

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/module"  // Updated import path
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
)

// DeployAppPipeline is an example pipeline for deploying an application.
type DeployAppPipeline struct {
	modules []module.Module
}

// NewDeployAppPipeline creates a new DeployAppPipeline.
// It initializes its modules, in this case, with a WebServerModule.
func NewDeployAppPipeline() Pipeline {
	return &DeployAppPipeline{
		modules: []module.Module{
			module.NewWebServerModule(), // Assumes NewWebServerModule is available in package module
		},
	}
}

func (p *DeployAppPipeline) Name() string {
	return "DeployAppPipeline"
}

func (p *DeployAppPipeline) Modules() []module.Module {
	return p.modules
}

// Plan generates the execution plan for all relevant modules within this pipeline.
func (p *DeployAppPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionPlan, error) {
	totalPlan := &plan.ExecutionPlan{Phases: []plan.Phase{}}

	// The issue description implies PipelineContext can be asserted to ModuleContext.
	// Similar to Module -> Task context, this might need refinement in runtime design.
	// For now, proceeding with the assumption from the issue.
	moduleCtx, ok := ctx.(runtime.ModuleContext)
	if !ok {
		// This is a critical design point. If PipelineContext is not a ModuleContext,
		// then the way modules get their specific context needs to be defined.
		// For example, ctx.NewModuleContext(module) or similar.
		return nil, fmt.Errorf("unable to assert PipelineContext to ModuleContext for pipeline %s; context design needs review", p.Name())
	}

	for _, mod := range p.Modules() {
		// Here, we'd typically check if a module is "active" or "required"
		// based on some conditions or configuration within the PipelineContext if needed.
		// For this example, we assume all modules in the pipeline are processed.
		// ctx.GetLogger().Infof("Planning for module %s in pipeline %s...", mod.Name(), p.Name())

		modulePlan, err := mod.Plan(moduleCtx) // Pass the asserted/derived ModuleContext
		if err != nil {
			return nil, fmt.Errorf("planning failed for module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
		}

		if modulePlan != nil && len(modulePlan.Phases) > 0 {
			totalPlan.Phases = append(totalPlan.Phases, modulePlan.Phases...)
		}
	}
	return totalPlan, nil
}

// Run executes the pipeline.
// It first generates the plan and then instructs the engine (via runtime.Context) to execute it.
func (p *DeployAppPipeline) Run(ctx *runtime.Context, dryRun bool) (*plan.ExecutionResult, error) {
	// ctx.GetLogger().Infof("Generating plan for pipeline %s...", p.Name())
	// The Plan method of the pipeline needs a runtime.PipelineContext.
	// The top-level runtime.Context should be able to provide this.

	if ctx == nil {
		// This check might be optional if ctx is guaranteed not to be nil by the caller (e.g., main)
		// but good for robustness within the Run method itself.
		return nil, fmt.Errorf("runtime.Context is nil for pipeline %s", p.Name())
	}
	// Since *runtime.Context implements runtime.PipelineContext (verified by static assertion),
	// it can be passed directly to methods expecting runtime.PipelineContext.
	totalPlan, err := p.Plan(ctx)
	if err != nil {
		// ctx.GetLogger().Errorf("Failed to generate plan for pipeline %s: %v", p.Name(), err)
		return nil, fmt.Errorf("failed to generate plan for pipeline %s: %w", p.Name(), err)
	}

	// ctx.GetLogger().Infof("Executing plan for pipeline %s (DryRun: %t)...", p.Name(), dryRun)
	// The Engine is accessed via the main runtime.Context.
	return ctx.Engine.Execute(ctx, totalPlan, dryRun)
}
