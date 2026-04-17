package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ExtractArchiveStep extracts an archive file.
type ExtractArchiveStep struct {
	step.Base
	ArchivePath     string
	TargetDir       string
	StripComponents int
}

type ExtractArchiveStepBuilder struct {
	step.Builder[ExtractArchiveStepBuilder, *ExtractArchiveStep]
}

func NewExtractArchiveStepBuilder(ctx runtime.ExecutionContext, instanceName, archivePath, targetDir string) *ExtractArchiveStepBuilder {
	s := &ExtractArchiveStep{
		ArchivePath:     archivePath,
		TargetDir:       targetDir,
		StripComponents: 0,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract %s to %s", instanceName, archivePath, targetDir)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(ExtractArchiveStepBuilder).Init(s)
}

func (b *ExtractArchiveStepBuilder) WithStripComponents(components int) *ExtractArchiveStepBuilder {
	b.Step.StripComponents = components
	return b
}

func (s *ExtractArchiveStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractArchiveStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ExtractArchiveStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	var cmd string
	switch {
	case len(s.ArchivePath) > 4 && s.ArchivePath[len(s.ArchivePath)-4:] == ".tar":
		cmd = fmt.Sprintf("tar -xf %s -C %s", s.ArchivePath, s.TargetDir)
	case len(s.ArchivePath) > 7 && s.ArchivePath[len(s.ArchivePath)-7:] == ".tar.gz":
		cmd = fmt.Sprintf("tar -xzf %s -C %s", s.ArchivePath, s.TargetDir)
	case len(s.ArchivePath) > 4 && s.ArchivePath[len(s.ArchivePath)-4:] == ".tgz":
		cmd = fmt.Sprintf("tar -xzf %s -C %s", s.ArchivePath, s.TargetDir)
	case len(s.ArchivePath) > 4 && s.ArchivePath[len(s.ArchivePath)-4:] == ".tar":
		cmd = fmt.Sprintf("tar -xf %s -C %s --strip-components=%d", s.ArchivePath, s.TargetDir, s.StripComponents)
	case len(s.ArchivePath) > 8 && s.ArchivePath[len(s.ArchivePath)-8:] == ".tar.bz2":
		cmd = fmt.Sprintf("tar -xjf %s -C %s", s.ArchivePath, s.TargetDir)
	case len(s.ArchivePath) > 4 && s.ArchivePath[len(s.ArchivePath)-4:] == ".zip":
		cmd = fmt.Sprintf("unzip -q %s -d %s", s.ArchivePath, s.TargetDir)
	default:
		cmd = fmt.Sprintf("tar -xzf %s -C %s", s.ArchivePath, s.TargetDir)
	}

	logger.Infof("Running: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to extract archive")
		return result, err
	}

	logger.Infof("Archive extracted successfully to %s", s.TargetDir)
	result.MarkCompleted("Archive extracted")
	return result, nil
}

func (s *ExtractArchiveStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ExtractArchiveStep)(nil)
