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
	imageProvider := images.NewImageProvider(&ctx)
	hybridnetImage := imageProvider.GetImage("hybridnet")

	if hybridnetImage == nil {
		if ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeHybridnet) {
			ctx.GetLogger().Errorf("Error: Hybridnet is enabled but 'hybridnet' image is not found in BOM for K8s version %s\n %v", ctx.GetClusterConfig().Spec.Kubernetes.Version, os.Stderr)
		}
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

func (s *GenerateHybridnetValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeHybridnet) {
		return true, nil
	}
	return false, nil
}

func (s *GenerateHybridnetValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeHybridnet))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for hybridnet in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "hybridnet-values.yaml"), nil
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
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateHybridnetValuesStep)(nil)
