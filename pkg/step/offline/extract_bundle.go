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

type ExtractBundleStep struct {
	step.Base
	BundlePath     string
	DestinationDir string
	markerFilePath string
}

type ExtractBundleStepBuilder struct {
	step.Builder[ExtractBundleStepBuilder, *ExtractBundleStep]
}

func NewExtractBundleStepBuilder(ctx runtime.Context, instanceName string) *ExtractBundleStepBuilder {
	s := &ExtractBundleStep{
		DestinationDir: ctx.GetGlobalWorkDir(),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract offline asset bundle", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute

	s.markerFilePath = filepath.Join(s.DestinationDir, ".kubexm_extracted")

	b := new(ExtractBundleStepBuilder).Init(s)
	return b
}

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

	if _, err := os.Stat(s.markerFilePath); err == nil {
		logger.Info("Offline bundle appears to be already extracted. Skipping extraction.", "markerFile", s.markerFilePath)
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

	if err := helpers.ExtractTarGz(s.BundlePath, s.DestinationDir); err != nil {
		return fmt.Errorf("failed to extract offline bundle: %w", err)
	}

	if err := os.WriteFile(s.markerFilePath, []byte(time.Now().String()), 0644); err != nil {
		logger.Warn("Failed to write extraction marker file.", "error", err)
	}

	logger.Info("Offline bundle extracted successfully.")
	return nil
}

func (s *ExtractBundleStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for ExtractBundleStep is a no-op. The extracted files will not be removed automatically.")
	return nil
}

var _ step.Step = (*ExtractBundleStep)(nil)
