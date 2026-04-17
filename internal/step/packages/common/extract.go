package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ExtractPackagesStep extracts a package bundle.
type ExtractPackagesStep struct {
	step.Base
	ArchivePath string
	TargetDir   string
}

type ExtractPackagesStepBuilder struct {
	step.Builder[ExtractPackagesStepBuilder, *ExtractPackagesStep]
}

func NewExtractPackagesStepBuilder(ctx runtime.ExecutionContext, instanceName, archivePath, targetDir string) *ExtractPackagesStepBuilder {
	s := &ExtractPackagesStep{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract packages from %s to %s", instanceName, archivePath, targetDir)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	return new(ExtractPackagesStepBuilder).Init(s)
}

func (s *ExtractPackagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractPackagesStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ExtractPackagesStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("tar -xzf %s -C %s", s.ArchivePath, s.TargetDir)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to extract packages")
		return result, err
	}

	logger.Infof("Packages extracted successfully to %s", s.TargetDir)
	result.MarkCompleted("Packages extracted")
	return result, nil
}

func (s *ExtractPackagesStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ExtractPackagesStep)(nil)
