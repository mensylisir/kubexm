package flannel

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanFlannelNodeFilesStep struct {
	step.Base
}

type CleanFlannelNodeFilesStepBuilder struct {
	step.Builder[CleanFlannelNodeFilesStepBuilder, *CleanFlannelNodeFilesStep]
}

func NewCleanFlannelNodeFilesStepBuilder(ctx runtime.Context, instanceName string) *CleanFlannelNodeFilesStepBuilder {
	s := &CleanFlannelNodeFilesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup Flannel runtime files on the node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(CleanFlannelNodeFilesStepBuilder).Init(s)
	return b
}

func (s *CleanFlannelNodeFilesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanFlannelNodeFilesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	flannelRunDir := "/run/flannel"
	exists, err := runner.Exists(ctx.GoContext(), conn, flannelRunDir)
	if err != nil {
		return false, fmt.Errorf("failed to check for directory '%s': %w", flannelRunDir, err)
	}

	if !exists {
		logger.Info("Flannel runtime directory not found. Step is done.")
		return true, nil
	}

	logger.Info("Flannel runtime directory found. Cleanup is required.")
	return false, nil
}

func (s *CleanFlannelNodeFilesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	flannelRunDir := "/run/flannel"
	logger.Infof("Cleaning up Flannel runtime directory: %s", flannelRunDir)
	if err := runner.Remove(ctx.GoContext(), conn, flannelRunDir, s.Sudo, true); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Warnf("Failed to remove Flannel runtime directory '%s': %v", flannelRunDir, err)
		}
	}

	logger.Info("Flannel node file cleanup process finished.")
	return nil
}

func (s *CleanFlannelNodeFilesStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CleanFlannelNodeFilesStep)(nil)
