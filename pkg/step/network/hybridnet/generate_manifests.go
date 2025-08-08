package hybridnet

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateHybridnetValuesStep struct {
	step.Base
	ImageRegistry  string
	ImageTag       string
	DefaultNetwork *v1alpha1.HybridnetDefaultNetworkConfig
	Features       *v1alpha1.HybridnetFeaturesConfig
}

type GenerateHybridnetValuesStepBuilder struct {
	step.Builder[GenerateHybridnetValuesStepBuilder, *GenerateHybridnetValuesStep]
}

func NewGenerateHybridnetValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateHybridnetValuesStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeHybridnet) {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	hybridnetImage := imageProvider.GetImage("hybridnet")

	if hybridnetImage == nil {
		return nil
	}

	s := &GenerateHybridnetValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Hybridnet Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = hybridnetImage.FullNameWithoutTag()
	s.ImageTag = hybridnetImage.Tag()

	clusterCfg := ctx.GetClusterConfig()
	userHybridnetCfg := clusterCfg.Spec.Network.Hybridnet
	if userHybridnetCfg != nil {
		if userHybridnetCfg.Installation != nil && userHybridnetCfg.Installation.ImageRegistry != "" {
			s.ImageRegistry = userHybridnetCfg.Installation.ImageRegistry
		}
		s.DefaultNetwork = userHybridnetCfg.DefaultNetwork
		s.Features = userHybridnetCfg.Features
	}

	if s.DefaultNetwork == nil {
		s.DefaultNetwork = &v1alpha1.HybridnetDefaultNetworkConfig{}
	}
	if s.Features == nil {
		s.Features = &v1alpha1.HybridnetFeaturesConfig{}
	}

	b := new(GenerateHybridnetValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateHybridnetValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateHybridnetValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeHybridnet))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for hybridnet in BOM")
	}

	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())

	return filepath.Join(chartDir, chart.Version, valuesFileName), nil
}

func (s *GenerateHybridnetValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeHybridnet) {
		logger.Info("Hybridnet is not enabled, skipping.")
		return true, nil
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping step, could not determine values path: %v", err)
		return true, nil
	}

	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("Hybridnet values file %s already exists. Step is complete.", localPath)
		return true, nil
	}

	return false, nil
}

func (s *GenerateHybridnetValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("cni/hybridnet/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded hybridnet values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("hybridnetValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse hybridnet values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render hybridnet values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Hybridnet Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Hybridnet Helm values file.")
	return nil
}

func (s *GenerateHybridnetValuesStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*GenerateHybridnetValuesStep)(nil)
