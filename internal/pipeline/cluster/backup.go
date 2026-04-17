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

// BackupPipeline defines the pipeline for cluster backup operations.
type BackupPipeline struct {
	*pipeline.Base
	PipelineModules []module.Module
	BackupType      string
}

// NewBackupPipeline creates a new BackupPipeline.
func NewBackupPipeline(backupType string) pipeline.Pipeline {
	// Validate backupType
	validTypes := map[string]bool{"all": true, "pki": true, "etcd": true, "kubernetes": true}
	if !validTypes[backupType] {
		backupType = "all" // Safe default
	}

	return &BackupPipeline{
		Base:       pipeline.NewBase("Backup", fmt.Sprintf("Backup cluster data (%s)", backupType)),
		BackupType: backupType,
		PipelineModules: []module.Module{
			preflight.NewPreflightConnectivityModule(),
			backup.NewBackupModule(backupType),
		},
	}
}

func (p *BackupPipeline) Name() string             { return p.Base.Meta.Name }
func (p *BackupPipeline) Description() string      { return p.Base.Meta.Description }
func (p *BackupPipeline) Modules() []module.Module { return p.PipelineModules }

func (p *BackupPipeline) Plan(ctx runtime2.PipelineContext) (*plan.ExecutionGraph, error) {
	return pipeline.SafePlan(ctx, p.Name(), func() (*plan.ExecutionGraph, error) {
		logger := ctx.GetLogger().With("pipeline", p.Name())
		logger.Info("Planning backup pipeline...", "backup_type", p.BackupType)

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
		logger.Info("Backup pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
		return finalGraph, nil
	})
}

func (p *BackupPipeline) Run(ctx runtime2.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	clusterCfg := ctx.GetClusterConfig()
	workDir := "unknown"
	if clusterCfg != nil && clusterCfg.Spec.Global != nil {
		workDir = clusterCfg.Spec.Global.WorkDir
	}
	logger.Info("Running backup pipeline...", "dryRun", dryRun, "backup_type", p.BackupType, "workDir", workDir)

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
		logger.Info("Backup pipeline has no executable nodes.")
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

	logger.Info("Backup pipeline completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*BackupPipeline)(nil)
