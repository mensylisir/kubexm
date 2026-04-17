package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/module/etcd"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// UpgradeEtcdPipeline defines the pipeline for upgrading etcd in an existing Kubernetes cluster.
type UpgradeEtcdPipeline struct {
	*pipeline.Base
	TargetVersion   string
}

// NewUpgradeEtcdPipeline creates a new UpgradeEtcdPipeline.
func NewUpgradeEtcdPipeline(targetVersion string) pipeline.Pipeline {
	etcdModule := etcd.NewEtcdModule()

	return &UpgradeEtcdPipeline{
		Base:          pipeline.NewBase("UpgradeEtcd", "Upgrades etcd cluster to a target version"),
		TargetVersion: targetVersion,
		PipelineModules: []module.Module{
			preflight.NewPreflightConnectivityModule(),
			etcdModule,
		},
	}
}

func (p *UpgradeEtcdPipeline) Name() string {
	return p.Base.Meta.Name
}

func (p *UpgradeEtcdPipeline) Description() string {
	return p.Base.Meta.Description
}

func (p *UpgradeEtcdPipeline) Modules() []module.Module {
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

func (p *UpgradeEtcdPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name(), "target_version", p.TargetVersion)
	logger.Info("Planning pipeline for etcd upgrade...")

	// NOTE: Etcd upgrade is not fully implemented yet. The current EtcdModule only supports
	// deployment/installation, not rolling upgrade. Running this pipeline would incorrectly
	// attempt to install etcd rather than upgrade it.
	//
	// TODO: Implement a proper UpgradeEtcdModule with:
	// 1. DownloadEtcdStep for the target version
	// 2. Rolling upgrade steps (stop old -> install new -> start new) for each etcd node
	// 3. EtcdFinalizeUpgradeStep to set the cluster version
	return nil, fmt.Errorf("etcd upgrade pipeline is not fully implemented. The current implementation only supports deployment, not upgrade. Please implement a proper UpgradeEtcdModule before using this feature")
}

func (p *UpgradeEtcdPipeline) Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running etcd upgrade pipeline...", "dryRun", dryRun, "target_version", p.TargetVersion)

	engineCtx, ok := ctx.(*runtime.Context)
	if !ok {
		err := fmt.Errorf("pipeline context cannot be asserted to *runtime.Context for pipeline %s", p.Name())
		logger.With("error", err).Error("Context type assertion failed")
		return nil, err
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		logger.Info("No pre-computed graph provided to UpgradeEtcdPipeline.Run, planning now...")
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			logger.With("error", err).Error("Pipeline planning phase failed within Run method.")
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || currentGraph.IsEmpty() {
		logger.Info("Pipeline planned no executable nodes for etcd upgrade or was given an empty graph. Nothing to run.")
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
	logger.Info("Etcd upgrade pipeline run completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*UpgradeEtcdPipeline)(nil)
