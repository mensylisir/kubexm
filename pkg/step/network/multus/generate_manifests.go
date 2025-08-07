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
	imageProvider := images.NewImageProvider(&ctx)
	multusImage := imageProvider.GetImage("multus")

	if multusImage == nil {
		if *ctx.GetClusterConfig().Spec.Network.Multus.Installation.Enabled {
			ctx.GetLogger().Errorf("Error: Multus is enabled but 'multus' image is not found in BOM for K8s version %s\n %v", ctx.GetClusterConfig().Spec.Kubernetes.Version, os.Stderr)
		}
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

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network.Multus != nil &&
		clusterCfg.Spec.Network.Multus.Installation != nil &&
		clusterCfg.Spec.Network.Multus.Installation.Image != "" {

		fullImage := clusterCfg.Spec.Network.Multus.Installation.Image
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

func (s *GenerateMultusValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if !*ctx.GetClusterConfig().Spec.Network.Multus.Installation.Enabled {
		return true, nil
	}
	return false, nil
}

func (s *GenerateMultusValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeMultus))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for multus in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "multus-values.yaml"), nil
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
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateMultusValuesStep)(nil)
