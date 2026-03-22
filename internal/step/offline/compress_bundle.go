package offline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

type CompressBundleStep struct {
	step.Base
	SourceDir  string
	OutputPath string
}

type CompressBundleStepBuilder struct {
	step.Builder[CompressBundleStepBuilder, *CompressBundleStep]
}

func NewCompressBundleStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CompressBundleStepBuilder {
	s := &CompressBundleStep{
		SourceDir: ctx.GetGlobalWorkDir(),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Compress shared asset cache into an offline bundle", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute

	b := new(CompressBundleStepBuilder).Init(s)
	return b
}

func (b *CompressBundleStepBuilder) WithOutputPath(path string) *CompressBundleStepBuilder {
	b.Step.OutputPath = path
	return b
}

func (s *CompressBundleStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CompressBundleStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	if s.OutputPath == "" {
		return false, fmt.Errorf("output path for the offline bundle is not specified")
	}

	dirEntries, err := os.ReadDir(s.SourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("source directory for compression does not exist: %s", s.SourceDir)
		}
		return false, fmt.Errorf("failed to read source directory %s: %w", s.SourceDir, err)
	}
	if len(dirEntries) == 0 {
		return false, fmt.Errorf("source directory %s is empty, nothing to compress", s.SourceDir)
	}

	if _, err := os.Stat(s.OutputPath); err == nil {
		logger.Info("Offline bundle already exists at the output path. Skipping compression.", "path", s.OutputPath)
		return true, nil
	}

	return false, nil
}

func (s *CompressBundleStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())

	logger.Info("Compressing asset directory into an offline bundle...", "source", s.SourceDir, "output", s.OutputPath)

	outputParentDir := filepath.Dir(s.OutputPath)
	if err := os.MkdirAll(outputParentDir, 0755); err != nil {
		result.MarkFailed(err, "failed to create parent directory for output file")
		return result, err
	}

	if err := helpers.CompressTarGz(s.SourceDir, s.OutputPath); err != nil {
		result.MarkFailed(err, "failed to compress asset bundle")
		return result, err
	}

	logger.Info("Offline bundle created successfully.", "path", s.OutputPath)
	result.MarkCompleted("offline bundle compressed successfully")
	return result, nil
}

func (s *CompressBundleStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rolling back by deleting the created offline bundle...", "path", s.OutputPath)
	if err := os.Remove(s.OutputPath); err != nil && !os.IsNotExist(err) {
		logger.Error(err, "Failed to remove offline bundle during rollback.")
	}
	return nil
}

var _ step.Step = (*CompressBundleStep)(nil)
