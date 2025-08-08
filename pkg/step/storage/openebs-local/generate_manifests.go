package openebslocal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateOpenEBSValuesStep struct {
	step.Base
	ImageRegistry       string
	ImageTag            string
	NdmOperatorImageTag string
	NdmDaemonImageTag   string
}

type GenerateOpenEBSValuesStepBuilder struct {
	step.Builder[GenerateOpenEBSValuesStepBuilder, *GenerateOpenEBSValuesStep]
}

func NewGenerateOpenEBSValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateOpenEBSValuesStepBuilder {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Addons == nil || cfg.Spec.Storage.OpenEBS == nil || !*cfg.Spec.Storage.OpenEBS.Enabled {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	provisionerImage := imageProvider.GetImage("openebs-provisioner-localpv")
	ndmOperatorImage := imageProvider.GetImage("openebs-ndm-operator")
	ndmDaemonImage := imageProvider.GetImage("openebs-ndm")

	if provisionerImage == nil || ndmOperatorImage == nil || ndmDaemonImage == nil {
		ctx.GetLogger().Errorf("OpenEBS is enabled but one or more required images are not found in BOM for K8s version %s", cfg.Spec.Kubernetes.Version)
		return nil
	}

	s := &GenerateOpenEBSValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate OpenEBS Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = provisionerImage.RegistryAddrWithNamespace()
	s.ImageTag = provisionerImage.Tag()
	s.NdmOperatorImageTag = ndmOperatorImage.Tag()
	s.NdmDaemonImageTag = ndmDaemonImage.Tag()

	b := new(GenerateOpenEBSValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateOpenEBSValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateOpenEBSValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(OpenEBSChartName)
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for '%s' in BOM", OpenEBSChartName)
	}

	chartDownloadDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))
	return filepath.Join(chartDownloadDir, chart.Version, "openebs-values.yaml"), nil
}

func (s *GenerateOpenEBSValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Addons == nil || cfg.Spec.Storage.OpenEBS == nil || !*cfg.Spec.Storage.OpenEBS.Enabled {
		return true, nil
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return false, nil
	}
	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("OpenEBS values file %s already exists. Skipping generation.", localPath)
		return true, nil
	}

	return false, nil
}

func (s *GenerateOpenEBSValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("storage/openebs-local/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded openebs values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("openebsValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse openebs values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render openebs values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating OpenEBS Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated OpenEBS Helm values file.")
	return nil
}

func (s *GenerateOpenEBSValuesStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*GenerateOpenEBSValuesStep)(nil)
