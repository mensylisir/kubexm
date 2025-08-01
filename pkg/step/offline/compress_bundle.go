package offline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

// CompressBundleStep is a step to create an offline asset bundle.
type CompressBundleStep struct {
	step.Base
	SourceDir  string
	OutputPath string
}

// CompressBundleStepBuilder is used to build CompressBundleStep instances.
type CompressBundleStepBuilder struct {
	step.Builder[CompressBundleStepBuilder, *CompressBundleStep]
}

// NewCompressBundleStepBuilder is the constructor for CompressBundleStep.
func NewCompressBundleStepBuilder(ctx runtime.Context, instanceName string) *CompressBundleStepBuilder {
	s := &CompressBundleStep{
		// The source should be the root of the shared asset cache.
		SourceDir: ctx.GetGlobalWorkDir(),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Compress shared asset cache into an offline bundle", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute // Compression can take time

	b := new(CompressBundleStepBuilder).Init(s)
	return b
}

// WithOutputPath sets the path for the output tarball.
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

	// Check if the source directory exists and is not empty.
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

	// If the output file already exists, we can consider the step done.
	// This prevents re-compressing unnecessarily.
	if _, err := os.Stat(s.OutputPath); err == nil {
		logger.Info("Offline bundle already exists at the output path. Skipping compression.", "path", s.OutputPath)
		return true, nil
	}

	return false, nil
}

func (s *CompressBundleStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	logger.Info("Compressing asset directory into an offline bundle...", "source", s.SourceDir, "output", s.OutputPath)

	// Ensure the parent directory of the output path exists.
	outputParentDir := filepath.Dir(s.OutputPath)
	if err := os.MkdirAll(outputParentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for output file: %w", err)
	}

	// Assuming a helper function exists to handle tar.gz compression.
	// This would be the counterpart to ExtractTarGz.
	if err := helpers.CompressTarGz(s.SourceDir, s.OutputPath); err != nil {
		return fmt.Errorf("failed to compress asset bundle: %w", err)
	}

	logger.Info("Offline bundle created successfully.", "path", s.OutputPath)
	return nil
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
