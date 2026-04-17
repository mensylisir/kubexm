package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/module/backup"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
	runtime2 "github.com/mensylisir/kubexm/internal/runtime"
)

// RestorePipeline defines the pipeline for cluster restore operations.
type RestorePipeline struct {
	*pipeline.Base
	PipelineModules []module.Module
	RestoreType     string
	SnapshotPath    string
	AssumeYes       bool
}

// NewRestorePipeline creates a new RestorePipeline.
func NewRestorePipeline(restoreType, snapshotPath string, assumeYes bool) pipeline.Pipeline {
	// Validate restoreType
	validTypes := map[string]bool{"all": true, "pki": true, "etcd": true, "kubernetes": true}
	if !validTypes[restoreType] {
		restoreType = "all" // Safe default
	}

	return &RestorePipeline{
		Base:         pipeline.NewBase("Restore", fmt.Sprintf("Restore cluster data (%s)", restoreType)),
		RestoreType:  restoreType,
		SnapshotPath: snapshotPath,
		PipelineModules: []module.Module{
			preflight.NewPreflightConnectivityModule(), // SSH connectivity check
			preflight.NewPreflightModule(assumeYes),     // Connectivity + user confirmation (CRITICAL for restore)
			backup.NewRestoreModule(restoreType, snapshotPath),
		},
		AssumeYes: assumeYes,
	}
}

func (p *RestorePipeline) Name() string             { return p.Base.Meta.Name }
func (p *RestorePipeline) Description() string      { return p.Base.Meta.Description }
func (p *RestorePipeline) Modules() []module.Module { return p.PipelineModules }

func (p *RestorePipeline) Plan(ctx runtime2.PipelineContext) (*plan.ExecutionGraph, error) {
	return pipeline.SafePlan(ctx, p.Name(), func() (*plan.ExecutionGraph, error) {
		logger := ctx.GetLogger().With("pipeline", p.Name())
		logger.Info("Planning restore pipeline...", "restore_type", p.RestoreType)

		// Validation: snapshotPath is required for restore operations
		if p.SnapshotPath == "" {
			return nil, fmt.Errorf("snapshot path is required for restore pipeline (use --snapshot-path flag)")
		}

		finalGraph := plan.NewExecutionGraph(p.Name())
		var previousModuleExitNodes []plan.NodeID

		moduleCtx, ok := ctx.(runtime2.ModuleContext)
		if !ok {
			return nil, fmt.Errorf("pipeline context cannot be asserted to module.ModuleContext for pipeline %s", p.Name())
		}

		for _, mod := range p.Modules() {
			logger.Info("Planning module", "module", mod.Name())
			moduleFragment, err := pipeline.SafeModulePlan(moduleCtx, p.Name(), mod)
			if err != nil {
				return nil, fmt.Errorf("failed to plan module %s: %w", mod.Name(), err)
			}
			if moduleFragment.IsEmpty() {
				logger.Debug("Module produced empty fragment", "module", mod.Name())
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
		logger.Info("Restore pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
		return finalGraph, nil
	})
}

func (p *RestorePipeline) Run(ctx runtime2.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running restore pipeline...", "dryRun", dryRun, "restore_type", p.RestoreType, "snapshotPath", p.SnapshotPath)

	engineCtx, ok := ctx.(*runtime2.Context)
	if !ok {
		return nil, fmt.Errorf("pipeline context cannot be asserted to *runtime2.Context")
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || currentGraph.IsEmpty() {
		logger.Info("Restore pipeline has no executable nodes.")
		return &plan.GraphExecutionResult{
			GraphName: p.Name(),
			Status:    plan.StatusSuccess,
		}, nil
	}

	execEngine := engine.NewCheckpointExecutorForPipeline(engineCtx, p.Name())
	result, execErr := execEngine.Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		if result == nil {
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}

	logger.Info("Restore pipeline completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*RestorePipeline)(nil)
