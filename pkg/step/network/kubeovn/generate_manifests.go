package kubeovn

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

type GenerateKubeovnValuesStep struct {
	step.Base
	ImageRegistry    string
	ImageTag         string
	PodCIDR          string
	ServiceCIDR      string
	Networking       *v1alpha1.KubeOvnNetworking
	Controller       *v1alpha1.KubeOvnControllerConfig
	AdvancedFeatures *v1alpha1.KubeOvnAdvancedFeatures
}

type GenerateKubeovnValuesStepBuilder struct {
	step.Builder[GenerateKubeovnValuesStepBuilder, *GenerateKubeovnValuesStep]
}

func NewGenerateKubeovnValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeovnValuesStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	kubeovnImage := imageProvider.GetImage("kube-ovn")

	if kubeovnImage == nil {
		return nil
	}

	s := &GenerateKubeovnValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Kube-OVN Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = kubeovnImage.FullNameWithoutTag()
	s.ImageTag = kubeovnImage.Tag()

	clusterCfg := ctx.GetClusterConfig()
	s.ServiceCIDR = clusterCfg.Spec.Network.KubeServiceCIDR
	s.PodCIDR = clusterCfg.Spec.Network.KubePodsCIDR

	userKubeOvnCfg := clusterCfg.Spec.Network.KubeOvn
	if userKubeOvnCfg != nil {
		s.Networking = userKubeOvnCfg.Networking
		s.Controller = userKubeOvnCfg.Controller
		s.AdvancedFeatures = userKubeOvnCfg.AdvancedFeatures

		if userKubeOvnCfg.Controller != nil && userKubeOvnCfg.Controller.PodDefaultSubnetCIDR != "" {
			s.PodCIDR = userKubeOvnCfg.Controller.PodDefaultSubnetCIDR
		}
	}

	if s.Networking == nil {
		s.Networking = &v1alpha1.KubeOvnNetworking{}
	}
	if s.Networking.TunnelType == "" {
		s.Networking.TunnelType = "geneve"
	}
	if s.Controller == nil {
		s.Controller = &v1alpha1.KubeOvnControllerConfig{
			JoinCIDR:       "100.64.0.0/16",
			NodeSwitchCIDR: "192.168.0.0/24",
		}
	}

	b := new(GenerateKubeovnValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeovnValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeovnValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for kube-ovn in BOM")
	}

	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())

	return filepath.Join(chartDir, chart.Version, valuesFileName), nil
}

func (s *GenerateKubeovnValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		logger.Info("Kube-OVN is not enabled, skipping.")
		return true, nil
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping step, could not determine values path: %v", err)
		return true, nil
	}

	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("Kube-OVN values file %s already exists. Step is complete.", localPath)
		return true, nil
	}

	return false, nil
}

func (s *GenerateKubeovnValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("cni/kubeovn/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded kube-ovn values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("kubeovnValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse kube-ovn values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render kube-ovn values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Kube-OVN Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Kube-OVN Helm values file.")
	return nil
}

func (s *GenerateKubeovnValuesStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*GenerateKubeovnValuesStep)(nil)
