package longhorn

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/util/helm"
	"github.com/mensylisir/kubexm/internal/util/images"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type GenerateLonghornValuesStep struct {
	step.Base
	SystemDefaultRegistry string
}

type GenerateLonghornValuesStepBuilder struct {
	step.Builder[GenerateLonghornValuesStepBuilder, *GenerateLonghornValuesStep]
}

func NewGenerateLonghornValuesStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateLonghornValuesStepBuilder {
	if ctx.GetClusterConfig().Spec.Addons == nil || ctx.GetClusterConfig().Spec.Storage.Longhorn == nil || !*ctx.GetClusterConfig().Spec.Storage.Longhorn.Enabled {
		return nil
	}

	imageProvider := images.NewImageProvider(ctx)
	longhornImage := imageProvider.GetImage("longhorn-manager")
	if longhornImage == nil {
		ctx.GetLogger().Errorf("Error: Longhorn is enabled but 'longhorn-manager' image is not found in BOM for K8s version %s\n %v", ctx.GetClusterConfig().Spec.Kubernetes.Version, os.Stderr)
		return nil
	}

	s := &GenerateLonghornValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Generate Longhorn Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.SystemDefaultRegistry = longhornImage.RegistryAddrWithNamespace()

	b := new(GenerateLonghornValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateLonghornValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateLonghornValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Addons == nil || cfg.Spec.Storage.Longhorn == nil || !*cfg.Spec.Storage.Longhorn.Enabled {
		return true, nil
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return false, nil
	}

	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("Longhorn values file %s already exists. Skipping generation.", localPath)
		return true, nil
	}
	return false, nil
}

func (s *GenerateLonghornValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("longhorn")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for longhorn in BOM")
	}
	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))
	return filepath.Join(chartDir, chart.Version, "longhorn-values.yaml"), nil
}

func (s *GenerateLonghornValuesStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("storage/longhorn/values.yaml.tmpl")
	if err != nil {
		result.MarkFailed(err, "failed to get embedded longhorn values.yaml.tmpl")
		return result, err
	}

	tmpl, err := template.New("longhornValues").Parse(valuesTemplateContent)
	if err != nil {
		result.MarkFailed(err, "failed to parse longhorn values.yaml.tmpl")
		return result, err
	}

	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		result.MarkFailed(err, "failed to render longhorn values.yaml.tmpl")
		return result, err
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to get local values path")
		return result, err
	}

	logger.Infof("Generating Longhorn Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		result.MarkFailed(err, "failed to create local directory for values file")
		return result, err
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		result.MarkFailed(err, "failed to write generated values file")
		return result, err
	}

	logger.Info("Successfully generated Longhorn Helm values file.")
	result.MarkCompleted("Longhorn values file generated successfully")
	return result, nil
}

func (s *GenerateLonghornValuesStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*GenerateLonghornValuesStep)(nil)
