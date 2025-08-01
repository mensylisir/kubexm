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

// ExtractBundleStep is a step to extract an offline asset bundle.
type ExtractBundleStep struct {
	step.Base
	BundlePath    string
	DestinationDir string
}

// ExtractBundleStepBuilder is used to build ExtractBundleStep instances.
type ExtractBundleStepBuilder struct {
	step.Builder[ExtractBundleStepBuilder, *ExtractBundleStep]
}

// NewExtractBundleStepBuilder is the constructor for ExtractBundleStep.
func NewExtractBundleStepBuilder(ctx runtime.Context, instanceName string) *ExtractBundleStepBuilder {
	s := &ExtractBundleStep{
		// The destination should be the root of the shared asset cache.
		DestinationDir: ctx.GetGlobalWorkDir(),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract offline asset bundle", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute // Extraction can take time

	b := new(ExtractBundleStepBuilder).Init(s)
	return b
}

// WithBundlePath sets the path to the offline bundle tarball.
func (b *ExtractBundleStepBuilder) WithBundlePath(path string) *ExtractBundleStepBuilder {
	b.Step.BundlePath = path
	return b
}

func (s *ExtractBundleStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractBundleStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	if s.BundlePath == "" {
		return false, fmt.Errorf("offline bundle path is not specified")
	}

	if _, err := os.Stat(s.BundlePath); os.IsNotExist(err) {
		return false, fmt.Errorf("offline bundle not found at specified path: %s", s.BundlePath)
	}

	// We could add a check here to see if the destination directory already contains
	// the expected assets, but for now, we'll assume extraction is desired if the
	// bundle exists. A simple check could be for a marker file.
	markerFile := filepath.Join(s.DestinationDir, ".kubexm-extracted")
	if _, err := os.Stat(markerFile); err == nil {
		logger.Info("Offline bundle appears to be already extracted. Skipping extraction.", "markerFile", markerFile)
		return true, nil
	}

	return false, nil
}

func (s *ExtractBundleStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	logger.Info("Extracting offline bundle...", "source", s.BundlePath, "destination", s.DestinationDir)

	if err := os.MkdirAll(s.DestinationDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", s.DestinationDir, err)
	}

	// Assuming a helper function exists to handle tar.gz extraction.
	// Based on the codebase structure, such a helper would likely be in `pkg/step/helpers`.
	if err := helpers.ExtractTarGz(s.BundlePath, s.DestinationDir); err != nil {
		return fmt.Errorf("failed to extract offline bundle: %w", err)
	}

	// Create a marker file to indicate successful extraction for future Prechecks.
	markerFile := filepath.Join(s.DestinationDir, ".kubexm-extracted")
	if err := os.WriteFile(markerFile, []byte(time.Now().String()), 0644); err != nil {
		// This is not a critical failure, so we just log a warning.
		logger.Warn("Failed to write extraction marker file.", "error", err)
	}

	logger.Info("Offline bundle extracted successfully.")
	return nil
}

func (s *ExtractBundleStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for ExtractBundleStep is a no-op. The extracted files will not be removed automatically.")
	// Automatically removing the entire shared cache on rollback is too destructive.
	// A more sophisticated rollback might remove only the files listed in the tarball's manifest,
	// but that requires a more complex implementation.
	return nil
}

var _ step.Step = (*ExtractBundleStep)(nil)
