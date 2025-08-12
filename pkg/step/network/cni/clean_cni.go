package cni

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanCNIStep struct {
	step.Base
}

type CleanCNIStepBuilder struct {
	step.Builder[CleanCNIStepBuilder, *CleanCNIStep]
}

func NewCleanCNIStepBuilder(ctx runtime.Context, instanceName string) *CleanCNIStepBuilder {
	s := &CleanCNIStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup CNI directories", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(CleanCNIStepBuilder).Init(s)
	return b
}

func (s *CleanCNIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanCNIStep) filesAndDirsToRemove() []string {
	paths := []string{
		common.DefaultCNIConfDirTarget,
		common.DefaultCNIBin,
	}
	return paths
}

func (s *CleanCNIStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	paths := s.filesAndDirsToRemove()
	for _, path := range paths {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, err
		}
		if exists {
			logger.Infof("CNI path '%s' still exists. Cleanup is required.", path)
			return false, nil
		}
	}

	logger.Info("All CNI related directories have been removed. Step is done.")
	return true, nil
}

func (s *CleanCNIStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	paths := s.filesAndDirsToRemove()

	for _, path := range paths {
		logger.Warnf("Removing CNI path: %s", path)
		if err := runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				logger.Errorf("Failed to remove '%s', manual cleanup may be required. Error: %v", path, err)
			}
		}
	}

	logger.Info("Successfully cleaned CNI directories.")
	return nil
}

func (s *CleanCNIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}

var _ step.Step = (*CleanCNIStep)(nil)
