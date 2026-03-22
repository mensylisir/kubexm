package registry

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RemoveRegistryDataStep struct {
	step.Base
	DataRoot string
}

type RemoveRegistryDataStepBuilder struct {
	step.Builder[RemoveRegistryDataStepBuilder, *RemoveRegistryDataStep]
}

func NewRemoveRegistryDataStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveRegistryDataStepBuilder {
	dataRoot := "/var/lib/registry"
	if localCfg := ctx.GetClusterConfig().Spec.Registry.LocalDeployment; localCfg != nil && localCfg.DataRoot != "" {
		dataRoot = localCfg.DataRoot
	}

	s := &RemoveRegistryDataStep{
		DataRoot: dataRoot,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>DANGER: Remove registry data directory", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute

	b := new(RemoveRegistryDataStepBuilder).Init(s)
	return b
}

func (s *RemoveRegistryDataStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveRegistryDataStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	exists, _ := runner.Exists(ctx.GoContext(), conn, s.DataRoot)
	return !exists, nil
}

func (s *RemoveRegistryDataStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Warnf("DANGER: Removing all registry data from '%s'. This will delete all pushed images.", s.DataRoot)
	time.Sleep(5 * time.Second)

	if err := runner.Remove(ctx.GoContext(), conn, s.DataRoot, s.Sudo, true); err != nil {
		result.MarkFailed(err, "failed to remove registry data")
		return result, err
	}
	result.MarkCompleted("registry data removed successfully")
	return result, nil
}

func (s *RemoveRegistryDataStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Error(nil, "FATAL: Rollback for registry data removal is impossible.")
	return nil
}

var _ step.Step = (*RemoveRegistryDataStep)(nil)
