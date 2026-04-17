package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	modulePki "github.com/mensylisir/kubexm/internal/module/pki"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
	runtime2 "github.com/mensylisir/kubexm/internal/runtime"
)

// RenewEtcdCAPipeline defines the pipeline for renewing all etcd CA certificates.
type RenewEtcdCAPipeline struct {
	*pipeline.Base
	PipelineModules []module.Module
	AssumeYes       bool
}

// NewRenewEtcdCAPipeline creates a new RenewEtcdCAPipeline.
func NewRenewEtcdCAPipeline(assumeYes bool) pipeline.Pipeline {
	modules := []module.Module{
		preflight.NewPreflightConnectivityModule(), // SSH connectivity check
		preflight.NewPreflightModule(assumeYes), // Connectivity + user confirmation (CRITICAL for cert renewal)
		modulePki.NewPKIModule("etcd-ca"),
	}

	return &RenewEtcdCAPipeline{
		Base:            pipeline.NewBase("RenewEtcdCA", "Renews all etcd CA certificates (CA + leaf certs) in the cluster"),
		PipelineModules: modules,
		AssumeYes:       assumeYes,
	}
}

func (p *RenewEtcdCAPipeline) Name() string {
	return p.Base.Meta.Name
}

func (p *RenewEtcdCAPipeline) Description() string {
	return p.Base.Meta.Description
}

func (p *RenewEtcdCAPipeline) Modules() []module.Module {
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

func (p *RenewEtcdCAPipeline) Plan(ctx runtime2.PipelineContext) (*plan.ExecutionGraph, error) {
	return pipeline.SafePlan(ctx, p.Name(), func() (*plan.ExecutionGraph, error) {
		logger := ctx.GetLogger().With("pipeline", p.Name())
		logger.Info("Planning pipeline for etcd CA certificate renewal...")

		finalGraph := plan.NewExecutionGraph(p.Name())
		var previousModuleExitNodes []plan.NodeID

		moduleCtx, ok := ctx.(runtime2.ModuleContext)
		if !ok {
			return nil, fmt.Errorf("pipeline context cannot be asserted to module.ModuleContext for pipeline %s", p.Name())
		}

		for i, mod := range p.Modules() {
			logger.Info("Planning module", "module_name", mod.Name(), "module_index", i)
			moduleFragment, err := pipeline.SafeModulePlan(moduleCtx, p.Name(), mod)
			if err != nil {
				return nil, fmt.Errorf("failed to plan module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
			}
			if moduleFragment.IsEmpty() {
				logger.Info("Module returned an empty fragment, skipping.", "module_name", mod.Name())
				continue
			}
			if err := finalGraph.MergeFragment(moduleFragment); err != nil {
				return nil, fmt.Errorf("failed to merge fragment from module %s: %w", mod.Name(), err)
			}

			if len(previousModuleExitNodes) > 0 {
				if err := plan.LinkFragments(finalGraph, previousModuleExitNodes, moduleFragment.EntryNodes); err != nil {
					return nil, fmt.Errorf("failed to link fragments in pipeline %s: %w", p.Name(), err)
				}
			}
			previousModuleExitNodes = moduleFragment.ExitNodes
		}

		finalGraph.CalculateEntryAndExitNodes()
		if err := finalGraph.Validate(); err != nil {
			return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
		}
		logger.Info("Etcd CA renewal pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
		return finalGraph, nil
	})
}

func (p *RenewEtcdCAPipeline) Run(ctx runtime2.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running etcd CA renewal pipeline...", "dryRun", dryRun)

	engineCtx, ok := ctx.(*runtime2.Context)
	if !ok {
		err := fmt.Errorf("pipeline context cannot be asserted to *runtime2.Context for pipeline %s", p.Name())
		logger.With("error", err).Error("Context type assertion failed")
		return nil, err
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		logger.Info("No pre-computed graph provided, planning now...")
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			logger.With("error", err).Error("Pipeline planning phase failed within Run method.")
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || currentGraph.IsEmpty() {
		logger.Info("Pipeline planned no executable nodes. Nothing to run.")
		return &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusSuccess}, nil
	}

	execEngine := engine.NewCheckpointExecutorForPipeline(engineCtx, p.Name())
	result, execErr := execEngine.Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		if result == nil {
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}
	logger.Info("Etcd CA renewal pipeline run completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*RenewEtcdCAPipeline)(nil)
