package cilium

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateCiliumValuesStep struct {
	step.Base
	ImageRepository         string
	ImageTag                string
	OperatorImageRepository string
	OperatorImageTag        string
	Tunnel                  string
	IpamMode                string
	KubeProxyReplacement    string
	BpfMasquerade           bool
	HubbleEnabled           bool
	HubbleUiEnabled         bool
	HubbleRelayEnabled      bool
	IdentityAllocationMode  string
	EncryptionEnabled       bool
	BandwidthManagerEnabled bool
	AutoDirectNodeRoutes    bool
	OperatorReplicas        int
}

type GenerateCiliumValuesStepBuilder struct {
	step.Builder[GenerateCiliumValuesStepBuilder, *GenerateCiliumValuesStep]
}

func NewGenerateCiliumValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateCiliumValuesStepBuilder {
	s := &GenerateCiliumValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Cilium Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	clusterCfg := ctx.GetClusterConfig()
	imageProvider := images.NewImageProvider(&ctx)
	ciliumImage := imageProvider.GetImage("cilium")
	operatorImage := imageProvider.GetImage("cilium-operator-generic")
	if ciliumImage == nil || operatorImage == nil {
		if clusterCfg.Spec.Network.Plugin == string(common.CNITypeCilium) {
			ctx.GetLogger().Errorf("Fatal: Cilium is enabled but its images are not found in BOM for K8s version %s.\n %v", clusterCfg.Spec.Kubernetes.Version, os.Stderr)
		}
		return nil
	}
	s.ImageRepository = ciliumImage.FullNameWithoutTag()
	s.ImageTag = ciliumImage.Tag()
	s.OperatorImageRepository = operatorImage.FullNameWithoutTag()
	s.OperatorImageTag = operatorImage.Tag()
	s.Tunnel = "vxlan"
	s.IpamMode = "kubernetes"
	s.KubeProxyReplacement = "probe"
	s.BpfMasquerade = true
	s.HubbleEnabled = true
	s.HubbleUiEnabled = true
	s.IdentityAllocationMode = "crd"
	s.EncryptionEnabled = false
	s.BandwidthManagerEnabled = false
	s.OperatorReplicas = 1
	s.HubbleRelayEnabled = true
	s.AutoDirectNodeRoutes = false
	userCiliumCfg := clusterCfg.Spec.Network.Cilium
	if userCiliumCfg != nil {
		if userCiliumCfg.Network != nil {
			if userCiliumCfg.Network.TunnelingMode != "" {
				s.Tunnel = userCiliumCfg.Network.TunnelingMode
			}
			if userCiliumCfg.Network.IPAMMode != "" {
				s.IpamMode = userCiliumCfg.Network.IPAMMode
			}
		}
		if userCiliumCfg.KubeProxy != nil {
			if userCiliumCfg.KubeProxy.ReplacementMode != "" {
				s.KubeProxyReplacement = userCiliumCfg.KubeProxy.ReplacementMode
			}
			if userCiliumCfg.KubeProxy.EnableBPFMasquerade != nil {
				s.BpfMasquerade = *userCiliumCfg.KubeProxy.EnableBPFMasquerade
			}
		}
		if userCiliumCfg.Hubble != nil {
			s.HubbleEnabled = userCiliumCfg.Hubble.Enable
			s.HubbleUiEnabled = userCiliumCfg.Hubble.EnableUI
		}
		if userCiliumCfg.Security != nil {
			if userCiliumCfg.Security.IdentityAllocationMode != "" {
				s.IdentityAllocationMode = userCiliumCfg.Security.IdentityAllocationMode
			}
			if userCiliumCfg.Security.EnableEncryption != nil {
				s.EncryptionEnabled = *userCiliumCfg.Security.EnableEncryption
			}
		}
		if userCiliumCfg.Performance != nil {
			if userCiliumCfg.Performance.EnableBandwidthManager != nil {
				s.BandwidthManagerEnabled = *userCiliumCfg.Performance.EnableBandwidthManager
			}
		}
	}

	b := new(GenerateCiliumValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateCiliumValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCiliumValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCilium) {
		return true, nil
	}
	return false, nil
}

func (s *GenerateCiliumValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeCilium))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for cilium in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "cilium-values.yaml"), nil
}

func (s *GenerateCiliumValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("cni/cilium/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded cilium values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("ciliumValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse cilium values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render cilium values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Cilium Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Cilium Helm values file.")
	return nil
}

func (s *GenerateCiliumValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateCiliumValuesStep)(nil)
