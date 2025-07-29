package registry

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

// RemoveRegistryDataStep 是一个无状态的节点执行步骤。
type RemoveRegistryDataStep struct {
	step.Base
	DataRoot string
}

type RemoveRegistryDataStepBuilder struct {
	step.Builder[RemoveRegistryDataStepBuilder, *RemoveRegistryDataStep]
}

func NewRemoveRegistryDataStepBuilder(ctx runtime.Context, instanceName string) *RemoveRegistryDataStepBuilder {
	dataRoot := "/var/lib/registry" // 默认
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

func (s *RemoveRegistryDataStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Warnf("DANGER: Removing all registry data from '%s'. This will delete all pushed images.", s.DataRoot)
	// 在实际产品中，这里可能需要一个额外的用户确认步骤
	time.Sleep(5 * time.Second) // 给用户一个取消的机会

	return runner.Remove(ctx.GoContext(), conn, s.DataRoot, s.Sudo, true)
}

func (s *RemoveRegistryDataStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Error(nil, "FATAL: Rollback for registry data removal is impossible.")
	return nil
}

var _ step.Step = (*RemoveRegistryDataStep)(nil)
