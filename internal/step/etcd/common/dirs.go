package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// PrepareEtcdDirsStep creates necessary directories for etcd.
type PrepareEtcdDirsStep struct {
	step.Base
	Dirs      []string
	Recursive bool
	Owner     string
	Group     string
}

type PrepareEtcdDirsStepBuilder struct {
	step.Builder[PrepareEtcdDirsStepBuilder, *PrepareEtcdDirsStep]
}

func NewPrepareEtcdDirsStepBuilder(ctx runtime.ExecutionContext, instanceName string, dirs []string) *PrepareEtcdDirsStepBuilder {
	s := &PrepareEtcdDirsStep{
		Dirs:      dirs,
		Recursive: true,
		Owner:     "root",
		Group:     "root",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Prepare etcd directories: %v", instanceName, dirs)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(PrepareEtcdDirsStepBuilder).Init(s)
}

func (b *PrepareEtcdDirsStepBuilder) WithRecursive(recursive bool) *PrepareEtcdDirsStepBuilder {
	b.Step.Recursive = recursive
	return b
}

func (b *PrepareEtcdDirsStepBuilder) WithOwner(owner, group string) *PrepareEtcdDirsStepBuilder {
	b.Step.Owner = owner
	b.Step.Group = group
	return b
}

func (s *PrepareEtcdDirsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PrepareEtcdDirsStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *PrepareEtcdDirsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	for _, dir := range s.Dirs {
		if err := runner.Mkdirp(ctx.GoContext(), conn, dir, "0755", s.Sudo); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to create directory %s", dir))
			return result, err
		}
		if s.Owner != "" && s.Group != "" {
			if err := runner.Chown(ctx.GoContext(), conn, dir, s.Owner, s.Group, s.Sudo); err != nil {
				logger.Warnf("Failed to set ownership on %s: %v", dir, err)
			}
		}
		logger.Infof("Directory %s created successfully", dir)
	}

	result.MarkCompleted(fmt.Sprintf("Prepared %d directories", len(s.Dirs)))
	return result, nil
}

func (s *PrepareEtcdDirsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*PrepareEtcdDirsStep)(nil)
