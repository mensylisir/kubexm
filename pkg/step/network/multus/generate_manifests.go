package multus

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateMultusValuesStep struct {
	step.Base
	ImageRepository string
	ImageTag        string
}

type GenerateMultusValuesStepBuilder struct {
	step.Builder[GenerateMultusValuesStepBuilder, *GenerateMultusValuesStep]
}

func NewGenerateMultusValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateMultusValuesStepBuilder {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus == nil || cfg.Spec.Network.Multus.Installation.Enabled == nil || !*cfg.Spec.Network.Multus.Installation.Enabled {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	multusImage := imageProvider.GetImage("multus")

	if multusImage == nil {
		return nil
	}

	s := &GenerateMultusValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Multus CNI Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRepository = multusImage.FullNameWithoutTag()
	s.ImageTag = multusImage.Tag()

	if cfg.Spec.Network.Multus != nil &&
		cfg.Spec.Network.Multus.Installation != nil &&
		cfg.Spec.Network.Multus.Installation.Image != "" {

		fullImage := cfg.Spec.Network.Multus.Installation.Image
		if lastColon := strings.LastIndex(fullImage, ":"); lastColon != -1 {
			if !strings.Contains(fullImage[lastColon:], "/") {
				s.ImageRepository = fullImage[:lastColon]
				s.ImageTag = fullImage[lastColon+1:]
			}
		}
	}

	b := new(GenerateMultusValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateMultusValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateMultusValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeMultus))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for multus in BOM")
	}

	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())

	return filepath.Join(chartDir, chart.Version, valuesFileName), nil
}

func (s *GenerateMultusValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus == nil || cfg.Spec.Network.Multus.Installation.Enabled == nil || !*cfg.Spec.Network.Multus.Installation.Enabled {
		logger.Info("Multus is not enabled, skipping.")
		return true, nil
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping step, could not determine values path: %v", err)
		return true, nil
	}

	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("Multus values file %s already exists. Step is complete.", localPath)
		return true, nil
	}

	return false, nil
}

func (s *GenerateMultusValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("cni/multus/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded multus values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("multusValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse multus values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render multus values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Multus Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Multus Helm values file.")
	return nil
}

func (s *GenerateMultusValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping rollback as no values path could be determined: %v", err)
		return nil
	}

	if _, statErr := os.Stat(localPath); statErr == nil {
		logger.Warnf("Rolling back by deleting generated values file: %s", localPath)
		if err := os.Remove(localPath); err != nil {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	} else {
		logger.Infof("Rollback unnecessary, file to be deleted does not exist: %s", localPath)
	}

	return nil
}

var _ step.Step = (*GenerateMultusValuesStep)(nil)
